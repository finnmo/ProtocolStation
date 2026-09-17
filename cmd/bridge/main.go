package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/coreos/go-systemd/v22/daemon"
	"go.uber.org/zap"

	"github.com/optech/protocol-bridge/internal/config"
	"github.com/optech/protocol-bridge/internal/health"
	"github.com/optech/protocol-bridge/internal/input"
	"github.com/optech/protocol-bridge/internal/logging"
	"github.com/optech/protocol-bridge/internal/output"
	"github.com/optech/protocol-bridge/internal/pipeline"
	"github.com/optech/protocol-bridge/internal/retry"
	"github.com/optech/protocol-bridge/internal/server"
	"github.com/optech/protocol-bridge/internal/server/modbus"
	"github.com/optech/protocol-bridge/internal/transformer"
)

func main() {
	var configFile string
	flag.StringVar(&configFile, "config", "config.yaml", "Configuration file path")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger with rotation support
	loggerConfig := &logging.LoggingConfig{
		Level:      cfg.Logging.Level,
		Format:     cfg.Logging.Format,
		File:       cfg.Logging.File,
		MaxSize:    cfg.Logging.MaxSize,
		MaxAge:     cfg.Logging.MaxAge,
		MaxBackups: cfg.Logging.MaxBackups,
		Compress:   cfg.Logging.Compress,
	}
	logger, err := logging.New(loggerConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	logger.Info("starting protocol bridge", zap.String("config", configFile))

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize health manager and HTTP server early so stats are available
	healthMgr := health.New()
	httpServer := server.NewHTTPServer(8080, healthMgr, logger)
	stats := httpServer.GetStats()

	// Start HTTP server for metrics and health checks in background
	httpCtx, httpCancel := context.WithCancel(ctx)
	defer httpCancel()

	go func() {
		if err := httpServer.Start(httpCtx); err != nil {
			logger.Error("HTTP server error", zap.Error(err))
		}
	}()

	// Initialize server manager
	serverMgr := server.NewManager(logger)
	for _, serverCfg := range cfg.Servers {
		serverMgr.AddServer(serverCfg)
	}

	// Start hosted servers
	if err := serverMgr.StartAll(ctx); err != nil {
		logger.Error("failed to start servers", zap.Error(err))
		os.Exit(1)
	}

	// Initialize retry manager and DLQ
	retryMgr := retry.NewRetryManager(cfg.ErrorHandling.Retry, logger)
	dlqMgr := retry.NewDLQManager(cfg.ErrorHandling.DLQ, logger)

	// Initialize DLQ
	if err := dlqMgr.Initialize(ctx); err != nil {
		logger.Error("failed to initialize DLQ", zap.Error(err))
		os.Exit(1)
	}

	// Create inputs
	inputs := make(map[string]input.Input)
	for _, inputCfg := range cfg.Inputs {
		switch inputCfg.Type {
		case "mqtt":
			in := input.NewMQTTInput(inputCfg, logger)
			inputs[inputCfg.Name] = in
		default:
			logger.Error("unsupported input type", zap.String("type", inputCfg.Type))
			os.Exit(1)
		}
	}

	// Create transformers
	transformers := make(map[string]transformer.Transformer)
	for _, transformerCfg := range cfg.Transformers {
		switch transformerCfg.Type {
		case "javascript":
			t, err := transformer.NewJavaScriptTransformer(transformerCfg, logger)
			if err != nil {
				logger.Error("failed to create JavaScript transformer",
					zap.String("name", transformerCfg.Name),
					zap.Error(err))
				os.Exit(1)
			}
			transformers[transformerCfg.Name] = t
		default:
			logger.Error("unsupported transformer type", zap.String("type", transformerCfg.Type))
			os.Exit(1)
		}
	}

	// Create outputs
	outputs := make(map[string]output.Output)
	for _, outputCfg := range cfg.Outputs {
		switch outputCfg.Type {
		case "mqtt":
			out := output.NewMQTTOutput(outputCfg, logger)
			outputs[outputCfg.Name] = out
		case "modbus":
			// Get the Modbus server context from the server manager
			var modbusCtx *modbus.ServerContext
			if outputCfg.Server != "" {
				if srv, exists := serverMgr.GetServer(outputCfg.Server); exists {
					modbusCtx = srv.GetModbusContext()
				}
			}
			if modbusCtx == nil {
				logger.Error("Modbus server context not available", zap.String("output", outputCfg.Name))
				os.Exit(1)
			}
			out := output.NewModbusOutput(outputCfg.Name, modbusCtx, logger)
			outputs[outputCfg.Name] = out
		default:
			logger.Error("unsupported output type", zap.String("type", outputCfg.Type))
			os.Exit(1)
		}
	}

	// Create and start pipelines
	var pipelines []*pipeline.Pipeline
	for _, pipelineCfg := range cfg.Pipelines {
		// Get input
		input, exists := inputs[pipelineCfg.Input]
		if !exists {
			logger.Error("pipeline references unknown input",
				zap.String("pipeline", pipelineCfg.Name),
				zap.String("input", pipelineCfg.Input))
			os.Exit(1)
		}

		// Process each route in the pipeline
		for _, route := range pipelineCfg.Routes {
			// Get transformer
			transformer, exists := transformers[route.Transformer]
			if !exists {
				logger.Error("pipeline references unknown transformer",
					zap.String("pipeline", pipelineCfg.Name),
					zap.String("transformer", route.Transformer))
				os.Exit(1)
			}

			// Get outputs
			var routeOutputs []output.Output
			for _, outputName := range route.Outputs {
				output, exists := outputs[outputName]
				if !exists {
					logger.Error("pipeline references unknown output",
						zap.String("pipeline", pipelineCfg.Name),
						zap.String("output", outputName))
					os.Exit(1)
				}
				routeOutputs = append(routeOutputs, output)
			}

			// Create pipeline for this route
			p := pipeline.NewPipeline(
				pipelineCfg,
				input,
				transformer,
				routeOutputs,
				retryMgr,
				dlqMgr,
				logger,
			)

			// Start pipeline
			if err := p.Start(ctx); err != nil {
				logger.Error("failed to start pipeline",
					zap.String("pipeline", pipelineCfg.Name),
					zap.Error(err))
				os.Exit(1)
			}

			// Set up stats recording for this pipeline
			p.SetStatsRecorder(func(action, component string) {
				// Record stats based on action type
				switch action {
				case "input_received":
					stats.RecordMessageReceived(pipelineCfg.Name, component)
				case "transformation_completed":
					stats.RecordTransformation(pipelineCfg.Name, component)
				case "output_sent":
					stats.RecordOutput(pipelineCfg.Name, component)
				}
			})

			pipelines = append(pipelines, p)
		}
	}

	logger.Info("protocol bridge started successfully",
		zap.Int("pipelines", len(pipelines)),
		zap.Int("inputs", len(inputs)),
		zap.Int("transformers", len(transformers)),
		zap.Int("outputs", len(outputs)))

	logger.Info("HTTP server available on port 8080",
		zap.String("metrics", "http://localhost:8080/metrics"),
		zap.String("health", "http://localhost:8080/health"),
		zap.String("ready", "http://localhost:8080/ready"),
		zap.String("status", "http://localhost:8080/status"))

	// Notify systemd that we're ready
	daemon.SdNotify(false, daemon.SdNotifyReady)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		logger.Info("received shutdown signal", zap.String("signal", sig.String()))
	case <-ctx.Done():
		logger.Info("context cancelled")
	}

	// Graceful shutdown
	logger.Info("shutting down protocol bridge...")

	// Stop pipelines
	for _, p := range pipelines {
		if err := p.Stop(); err != nil {
			logger.Error("failed to stop pipeline",
				zap.String("pipeline", p.Name()),
				zap.Error(err))
		}
	}

	// Stop DLQ
	if err := dlqMgr.Stop(); err != nil {
		logger.Error("failed to stop DLQ", zap.Error(err))
	}

	// Stop servers
	if err := serverMgr.StopAll(); err != nil {
		logger.Error("failed to stop servers", zap.Error(err))
	}

	logger.Info("protocol bridge stopped")
}
