package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"sync"

	"go.uber.org/zap"

	"github.com/optech/protocol-bridge/internal/health"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// HTTPServer handles HTTP endpoints for metrics and health checks
type HTTPServer struct {
	server    *http.Server
	manager   *health.HealthManager
	logger    *zap.Logger
	listening bool
	stats     *PipelineStats
}

// PipelineStats holds statistics for pipelines
type PipelineStats struct {
	Pipelines map[string]*PipelineInfo
	mu        sync.RWMutex
}

// PipelineInfo holds statistics for a single pipeline
type PipelineInfo struct {
	Input           PipelineComponent   `json:"input"`
	Transformations []PipelineComponent `json:"transformations"`
	Outputs         []PipelineComponent `json:"outputs"`
}

// PipelineComponent holds statistics for a component
type PipelineComponent struct {
	Name     string    `json:"name"`
	Type     string    `json:"type"`
	Count    int64     `json:"count"`
	LastTime time.Time `json:"last_time"`
}

// NewPipelineStats creates a new PipelineStats instance
func NewPipelineStats() *PipelineStats {
	return &PipelineStats{
		Pipelines: make(map[string]*PipelineInfo),
	}
}

// RecordMessageReceived records a message received
func (s *PipelineStats) RecordMessageReceived(pipeline, input string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Pipelines[pipeline] == nil {
		s.Pipelines[pipeline] = &PipelineInfo{
			Input:           PipelineComponent{Name: input, Type: "input"},
			Transformations: []PipelineComponent{},
			Outputs:         []PipelineComponent{},
		}
	}

	s.Pipelines[pipeline].Input.Count++
	s.Pipelines[pipeline].Input.LastTime = time.Now()
}

// RecordTransformation records a message transformation
func (s *PipelineStats) RecordTransformation(pipeline, transformer string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Pipelines[pipeline] == nil {
		s.Pipelines[pipeline] = &PipelineInfo{
			Input:           PipelineComponent{Name: "unknown", Type: "input"},
			Transformations: []PipelineComponent{},
			Outputs:         []PipelineComponent{},
		}
	}

	// Find or create transformer
	found := false
	for i, t := range s.Pipelines[pipeline].Transformations {
		if t.Name == transformer {
			s.Pipelines[pipeline].Transformations[i].Count++
			s.Pipelines[pipeline].Transformations[i].LastTime = time.Now()
			found = true
			break
		}
	}
	if !found {
		s.Pipelines[pipeline].Transformations = append(s.Pipelines[pipeline].Transformations, PipelineComponent{
			Name:     transformer,
			Type:     "transformer",
			Count:    1,
			LastTime: time.Now(),
		})
	}
}

// RecordOutput records an output message
func (s *PipelineStats) RecordOutput(pipeline, output string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Pipelines[pipeline] == nil {
		s.Pipelines[pipeline] = &PipelineInfo{
			Input:           PipelineComponent{Name: "unknown", Type: "input"},
			Transformations: []PipelineComponent{},
			Outputs:         []PipelineComponent{},
		}
	}

	// Find or create output
	found := false
	for i, o := range s.Pipelines[pipeline].Outputs {
		if o.Name == output {
			s.Pipelines[pipeline].Outputs[i].Count++
			s.Pipelines[pipeline].Outputs[i].LastTime = time.Now()
			found = true
			break
		}
	}
	if !found {
		s.Pipelines[pipeline].Outputs = append(s.Pipelines[pipeline].Outputs, PipelineComponent{
			Name:     output,
			Type:     "output",
			Count:    1,
			LastTime: time.Now(),
		})
	}
}

// GetStats returns the current statistics
func (s *PipelineStats) GetStats() map[string]*PipelineInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*PipelineInfo)
	for k, v := range s.Pipelines {
		result[k] = v
	}
	return result
}

// NewHTTPServer creates a new HTTP server
func NewHTTPServer(port int, manager *health.HealthManager, logger *zap.Logger) *HTTPServer {
	mux := http.NewServeMux()

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	h := &HTTPServer{
		server:  server,
		manager: manager,
		logger:  logger,
		stats:   NewPipelineStats(),
	}

	// Register handlers
	mux.HandleFunc("/health", h.healthHandler)
	mux.HandleFunc("/ready", h.readinessHandler)
	mux.HandleFunc("/status", h.statusHandler)
	mux.Handle("/metrics", promhttp.Handler())

	return h
}

// GetStats returns the stats instance
func (h *HTTPServer) GetStats() *PipelineStats {
	return h.stats
}

// Start starts the HTTP server
func (h *HTTPServer) Start(ctx context.Context) error {
	h.logger.Info("starting HTTP server",
		zap.String("addr", h.server.Addr))

	go func() {
		if err := h.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			h.logger.Error("HTTP server failed",
				zap.Error(err))
		}
	}()

	h.listening = true
	h.logger.Info("HTTP server started",
		zap.String("addr", h.server.Addr))

	// Wait for context cancellation
	<-ctx.Done()

	return h.Stop(ctx)
}

// Stop stops the HTTP server gracefully
func (h *HTTPServer) Stop(ctx context.Context) error {
	if !h.listening {
		return nil
	}

	h.logger.Info("stopping HTTP server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.server.Shutdown(shutdownCtx); err != nil {
		h.logger.Error("HTTP server shutdown failed",
			zap.Error(err))
		return err
	}

	h.listening = false
	h.logger.Info("HTTP server stopped")
	return nil
}

// healthHandler handles /health endpoint
func (h *HTTPServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	response := h.manager.Health()

	w.Header().Set("Content-Type", "application/json")

	if response.Status == health.StatusHealthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(response)
}

// readinessHandler handles /ready endpoint
func (h *HTTPServer) readinessHandler(w http.ResponseWriter, r *http.Request) {
	response := h.manager.Readiness()

	w.Header().Set("Content-Type", "application/json")

	if response.Status == health.StatusReady {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(response)
}

// statusHandler handles /status endpoint - provides readable pipeline statistics
func (h *HTTPServer) statusHandler(w http.ResponseWriter, r *http.Request) {
	stats := h.stats.GetStats()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	// Format response as requested by user
	type StatusResponse struct {
		Pipelines map[string]*PipelineInfo `json:"pipelines"`
	}

	response := StatusResponse{
		Pipelines: stats,
	}

	json.NewEncoder(w).Encode(response)
}
