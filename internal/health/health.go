package health

import (
	"encoding/json"
	"time"
)

// Status represents health status
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusReady     Status = "ready"
	StatusUnhealthy Status = "unhealthy"
	StatusNotReady  Status = "not_ready"
)

// ComponentStatus represents individual component status
type ComponentStatus string

const (
	ComponentConnected    ComponentStatus = "connected"
	ComponentConnecting   ComponentStatus = "connecting"
	ComponentDisconnected ComponentStatus = "disconnected"
	ComponentError        ComponentStatus = "error"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    Status    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// ReadinessResponse represents the readiness check response
type ReadinessResponse struct {
	Status     Status                   `json:"status"`
	Components map[string]ComponentInfo `json:"components"`
	Timestamp  time.Time                `json:"timestamp"`
}

// ComponentInfo represents status of a component
type ComponentInfo struct {
	Status ComponentStatus `json:"status"`
	Error  string          `json:"error,omitempty"`
}

// Checker interface for component health checking
type Checker interface {
	Check() ComponentStatus
	Error() error
}

// HealthManager manages health and readiness checks
type HealthManager struct {
	inputs    map[string]Checker
	outputs   map[string]Checker
	pipelines map[string]bool
}

// New creates a new HealthManager
func New() *HealthManager {
	return &HealthManager{
		inputs:    make(map[string]Checker),
		outputs:   make(map[string]Checker),
		pipelines: make(map[string]bool),
	}
}

// RegisterInput registers an input for health checking
func (h *HealthManager) RegisterInput(name string, checker Checker) {
	h.inputs[name] = checker
}

// RegisterOutput registers an output for health checking
func (h *HealthManager) RegisterOutput(name string, checker Checker) {
	h.outputs[name] = checker
}

// RegisterPipeline registers a pipeline for health checking
func (h *HealthManager) RegisterPipeline(name string) {
	h.pipelines[name] = true
}

// Health performs a liveness check
func (h *HealthManager) Health() HealthResponse {
	return HealthResponse{
		Status:    StatusHealthy,
		Timestamp: time.Now(),
	}
}

// Readiness performs a readiness check
func (h *HealthManager) Readiness() ReadinessResponse {
	components := make(map[string]ComponentInfo)

	// Check inputs
	for name, checker := range h.inputs {
		components["inputs."+name] = ComponentInfo{
			Status: checker.Check(),
			Error:  errorToString(checker.Error()),
		}
	}

	// Check outputs
	for name, checker := range h.outputs {
		components["outputs."+name] = ComponentInfo{
			Status: checker.Check(),
			Error:  errorToString(checker.Error()),
		}
	}

	// Check pipelines
	for name := range h.pipelines {
		components["pipelines."+name] = ComponentInfo{
			Status: ComponentConnected, // Assume ready if registered
		}
	}

	// Determine overall readiness
	status := StatusReady
	for _, info := range components {
		if info.Status == ComponentError {
			status = StatusNotReady
			break
		}
		if info.Status == ComponentDisconnected {
			// Check if all components are disconnected
			status = StatusNotReady
		}
	}

	return ReadinessResponse{
		Status:     status,
		Components: components,
		Timestamp:  time.Now(),
	}
}

// errorToString converts error to string safely
func errorToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// MarshalJSON implements json.Marshaler for HealthResponse
func (h HealthResponse) MarshalJSON() ([]byte, error) {
	type Alias HealthResponse
	return json.Marshal(Alias(h))
}

// MarshalJSON implements json.Marshaler for ReadinessResponse
func (r ReadinessResponse) MarshalJSON() ([]byte, error) {
	type Alias ReadinessResponse
	return json.Marshal(Alias(r))
}
