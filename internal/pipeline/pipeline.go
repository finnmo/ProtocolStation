package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/optech/protocol-bridge/internal/config"
	"github.com/optech/protocol-bridge/internal/input"
	"github.com/optech/protocol-bridge/internal/output"
	"github.com/optech/protocol-bridge/internal/retry"
	"github.com/optech/protocol-bridge/internal/transformer"
	"github.com/optech/protocol-bridge/pkg/message"
)

// Pipeline represents a message processing pipeline
type Pipeline struct {
	name           string
	config         config.PipelineConfig
	input          input.Input
	transformer    transformer.Transformer
	outputs        []output.Output
	retryMgr       *retry.RetryManager
	dlqMgr         *retry.DLQManager
	logger         *zap.Logger
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
	stats          func(action, component string) // stats recorder function
	circuitBreaker *CircuitBreaker                // circuit breaker for output operations
}

// NewPipeline creates a new pipeline
func NewPipeline(
	cfg config.PipelineConfig,
	input input.Input,
	transformer transformer.Transformer,
	outputs []output.Output,
	retryMgr *retry.RetryManager,
	dlqMgr *retry.DLQManager,
	logger *zap.Logger,
) *Pipeline {
	return &Pipeline{
		name:           cfg.Name,
		config:         cfg,
		input:          input,
		transformer:    transformer,
		outputs:        outputs,
		retryMgr:       retryMgr,
		dlqMgr:         dlqMgr,
		logger:         logger,
		circuitBreaker: NewCircuitBreaker(cfg.Name, 5, 30*time.Second),
	}
}

// Start begins processing messages through the pipeline
func (p *Pipeline) Start(ctx context.Context) error {
	p.ctx, p.cancel = context.WithCancel(ctx)

	// Start the input
	if err := p.input.Start(p.ctx); err != nil {
		return fmt.Errorf("failed to start input: %w", err)
	}

	// Start all outputs (non-blocking - log failures but continue)
	var failedOutputs []string
	for _, output := range p.outputs {
		if err := output.Start(p.ctx); err != nil {
			failedOutputs = append(failedOutputs, output.Name())
			p.logger.Error("failed to start output, will retry",
				zap.String("output", output.Name()),
				zap.Error(err))
		}
	}
	if len(failedOutputs) > 0 {
		p.logger.Warn("some outputs failed to start, pipeline will continue without them",
			zap.Strings("failed_outputs", failedOutputs))
	}

	// Start message processing goroutine
	p.wg.Add(1)
	go p.processMessages()

	p.logger.Info("pipeline started", zap.String("name", p.name))
	return nil
}

// Stop stops the pipeline
func (p *Pipeline) Stop() error {
	if p.cancel != nil {
		p.cancel()
	}

	// Stop input
	if err := p.input.Stop(); err != nil {
		p.logger.Error("failed to stop input", zap.Error(err))
	}

	// Stop outputs
	for _, output := range p.outputs {
		if err := output.Stop(); err != nil {
			p.logger.Error("failed to stop output",
				zap.String("output", output.Name()),
				zap.Error(err))
		}
	}

	// Wait for processing to complete
	p.wg.Wait()

	p.logger.Info("pipeline stopped", zap.String("name", p.name))
	return nil
}

// processMessages processes messages from the input
func (p *Pipeline) processMessages() {
	defer func() {
		if r := recover(); r != nil {
			p.logger.Error("panic in message processing, recovering",
				zap.Any("panic", r))
		}
		p.wg.Done()
	}()

	// Use a semaphore to limit concurrent processing and prevent stack overflow
	const maxConcurrent = 100
	semaphore := make(chan struct{}, maxConcurrent)

	for {
		select {
		case <-p.ctx.Done():
			return
		case msg, ok := <-p.input.Messages():
			if !ok {
				p.logger.Info("input channel closed")
				return
			}

			// Acquire semaphore to limit concurrency
			select {
			case semaphore <- struct{}{}:
				// Process the message in a goroutine with timeout
				go func(m *message.Message) {
					defer func() {
						<-semaphore // Release semaphore
						if r := recover(); r != nil {
							p.logger.Error("panic processing message",
								zap.String("message_id", m.ID),
								zap.Any("panic", r))
						}
					}()

					p.processMessage(m)
				}(msg)
			case <-p.ctx.Done():
				return
			}
		}
	}
}

// processMessage processes a single message through the pipeline
func (p *Pipeline) processMessage(msg *message.Message) {
	// Always record input message received (all messages, including filtered ones)
	if p.stats != nil {
		p.stats("input_received", p.input.Name())
	}

	// Transform the message
	transformedMsg, err := p.transformMessage(msg)
	if err != nil {
		p.logger.Error("failed to transform message",
			zap.String("message_id", msg.ID),
			zap.Error(err))

		// Send to DLQ
		if dlqErr := p.dlqMgr.SendToDLQ(p.ctx, msg, err); dlqErr != nil {
			p.logger.Error("failed to send message to DLQ",
				zap.String("message_id", msg.ID),
				zap.Error(dlqErr))
		}
		return
	}

	// If transform returns null, skip this message (filtered out) - silent skip
	if transformedMsg == nil {
		return
	}

	// Record transformation completed (only for non-filtered messages)
	if p.stats != nil {
		p.stats("transformation_completed", p.transformer.Name())
	}

	// Send to all outputs (if any)
	if len(p.outputs) == 0 {
		p.logger.Info("no outputs configured, skipping message send",
			zap.String("message_id", transformedMsg.ID))
		return
	}

	// Reduced logging to prevent stack overflow - only log at debug level
	p.logger.Debug("processing message",
		zap.String("message_id", transformedMsg.ID),
		zap.String("topic", msg.Topic),
		zap.String("source", msg.Source))

	for _, output := range p.outputs {
		p.sendToOutput(transformedMsg, output)
	}
}

// SetStatsRecorder sets the stats recorder for this pipeline
func (p *Pipeline) SetStatsRecorder(recorder func(action, component string)) {
	p.stats = recorder
}

// transformMessage transforms a message using the configured transformer
func (p *Pipeline) transformMessage(msg *message.Message) (*message.Message, error) {
	var transformed *message.Message
	err := p.retryMgr.ExecuteMessageWithRetry(p.ctx, msg, func(m *message.Message) error {
		var transformErr error
		transformed, transformErr = p.transformer.Transform(p.ctx, m)
		return transformErr
	}, fmt.Sprintf("transform (%s)", p.transformer.Name()))

	if err != nil {
		return nil, err
	}
	return transformed, nil
}

// sendToOutput sends a message to a specific output
func (p *Pipeline) sendToOutput(msg *message.Message, output output.Output) {
	// Use circuit breaker to protect against cascading failures
	var err error
	cbErr := p.circuitBreaker.Call(func() error {
		err = p.retryMgr.ExecuteMessageWithRetry(p.ctx, msg, func(m *message.Message) error {
			return output.Send(p.ctx, m)
		}, fmt.Sprintf("output (%s)", output.Name()))
		return err
	})

	// Circuit breaker is open, send directly to DLQ
	if _, ok := cbErr.(*ErrCircuitOpen); ok {
		p.logger.Warn("circuit breaker is open, sending message to DLQ",
			zap.String("message_id", msg.ID),
			zap.String("output", output.Name()))
		if dlqErr := p.dlqMgr.SendToDLQ(p.ctx, msg, cbErr); dlqErr != nil {
			p.logger.Error("failed to send message to DLQ",
				zap.String("message_id", msg.ID),
				zap.Error(dlqErr))
		}
		return
	}

	// Record output attempt
	if p.stats != nil {
		if err == nil {
			p.stats("output_sent", output.Name())
		}
	}

	if err != nil {
		p.logger.Error("failed to send message to output",
			zap.String("message_id", msg.ID),
			zap.String("output", output.Name()),
			zap.Error(err))

		// Send to DLQ
		if dlqErr := p.dlqMgr.SendToDLQ(p.ctx, msg, err); dlqErr != nil {
			p.logger.Error("failed to send message to DLQ",
				zap.String("message_id", msg.ID),
				zap.Error(dlqErr))
		}
	} else {
		// Reduced logging to prevent stack overflow
		p.logger.Debug("message sent to output successfully",
			zap.String("message_id", msg.ID),
			zap.String("output", output.Name()))
	}
}

// Name returns the pipeline name
func (p *Pipeline) Name() string {
	return p.name
}
