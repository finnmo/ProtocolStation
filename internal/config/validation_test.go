package config

import (
	"testing"
	"time"
)

func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Field:   "test.field",
		Value:   "invalid-value",
		Message: "test error",
		Hint:    "test hint",
	}

	msg := err.Error()
	if msg == "" {
		t.Error("Expected error message, got empty string")
	}
	if !expects(msg, "test.field", "test error", "test hint") {
		t.Errorf("Error message doesn't contain expected fields: %s", msg)
	}
}

func TestValidateBrokerURL(t *testing.T) {
	tests := []struct {
		name      string
		broker    string
		wantError bool
	}{
		{"valid simple", "localhost:1883", false},
		{"valid with protocol", "tls://broker.example.com:8883", false},
		{"valid ssl", "ssl://broker.example.com:8883", false},
		{"valid ws", "ws://broker.example.com:9001", false},
		{"valid wss", "wss://broker.example.com:9001", false},
		{"empty broker", "", true},
		{"invalid format", "invalid@broker:1883", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBrokerURL(tt.broker, "test.broker")
			if (err != nil) != tt.wantError {
				t.Errorf("validateBrokerURL(%q) error = %v, wantError %v", tt.broker, err, tt.wantError)
			}
		})
	}
}

func TestValidateClientID(t *testing.T) {
	tests := []struct {
		name      string
		clientID  string
		wantError bool
	}{
		{"valid alphanumeric", "client123", false},
		{"valid with hyphens", "client-123", false},
		{"valid with underscores", "client_123", false},
		{"valid with dots", "client.123", false},
		{"valid mixed", "client-123_abc.def", false},
		{"invalid space", "client 123", true},
		{"invalid special chars", "client@123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateClientID(tt.clientID)
			if (err != nil) != tt.wantError {
				t.Errorf("validateClientID(%q) error = %v, wantError %v", tt.clientID, err, tt.wantError)
			}
		})
	}
}

func TestValidateTopic(t *testing.T) {
	tests := []struct {
		name      string
		topic     string
		wantError bool
	}{
		{"valid simple", "sensors/temperature", false},
		{"valid with wildcard", "sensors/+", false},
		{"valid multi-level", "sensors/#", false},
		{"valid at end only", "sensors/#", false},
		{"valid with colon (Axis native event namespace)", "axis/TCP1/event/tns:axis/CameraApplicationPlatform/ObjectAnalytics/Device1Scenario1", false},
		{"empty topic", "", true},
		{"invalid # in middle", "sensors/#/temperature", true},
		{"invalid characters", "sensors/temp@123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTopic(tt.topic)
			if (err != nil) != tt.wantError {
				t.Errorf("validateTopic(%q) error = %v, wantError %v", tt.topic, err, tt.wantError)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	validConfig := &Config{
		Inputs: []InputConfig{
			{Name: "test-input", Type: "mqtt", Broker: "localhost:1883", ClientID: "client-1", Topics: []string{"test/topic"}, QoS: 1},
		},
		Transformers: []TransformerConfig{
			{Name: "test-transform", Type: "javascript", Script: "function transform(input) { return input; }"},
		},
		Outputs: []OutputConfig{
			{Name: "test-output", Type: "mqtt", Broker: "localhost:1883", ClientID: "client-2", Topic: "output/topic", QoS: 1},
		},
		Pipelines: []PipelineConfig{
			{
				Name:  "test-pipeline",
				Input: "test-input",
				Routes: []PipelineRoute{
					{Transformer: "test-transform", Outputs: []string{"test-output"}},
				},
			},
		},
		ErrorHandling: ErrorHandlingConfig{
			Retry: RetryConfig{Enabled: true, MaxAttempts: 3, InitialInterval: time.Second},
			DLQ:   DLQConfig{Enabled: true, Type: "mqtt", Broker: "localhost:1883", Topic: "dlq"},
		},
		Logging: LoggingConfig{Level: "info", Format: "json"},
	}

	if err := validateConfig(validConfig); err != nil {
		t.Errorf("Expected valid config to pass validation, got error: %v", err)
	}

	// Test duplicate input names
	duplicateInputs := &Config{
		Inputs: []InputConfig{
			{Name: "test-input", Type: "mqtt", Broker: "localhost:1883"},
			{Name: "test-input", Type: "mqtt", Broker: "localhost:1883"},
		},
	}
	if err := validateConfig(duplicateInputs); err == nil {
		t.Error("Expected validation error for duplicate input names")
	}

	// Test missing input in pipeline
	missingInput := &Config{
		Inputs: []InputConfig{
			{Name: "other-input", Type: "mqtt", Broker: "localhost:1883"},
		},
		Transformers: []TransformerConfig{
			{Name: "test-transform", Type: "javascript", Script: "function transform(input) { return input; }"},
		},
		Outputs: []OutputConfig{
			{Name: "test-output", Type: "mqtt", Broker: "localhost:1883", Topic: "output/topic", QoS: 1},
		},
		Pipelines: []PipelineConfig{
			{
				Name:  "test-pipeline",
				Input: "missing-input",
				Routes: []PipelineRoute{
					{Transformer: "test-transform", Outputs: []string{"test-output"}},
				},
			},
		},
	}
	if err := validateConfig(missingInput); err == nil {
		t.Error("Expected validation error for missing input reference in pipeline")
	}
}

func TestSplitHostPort(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHost string
		wantPort string
		wantErr  bool
	}{
		{"host only", "localhost", "localhost", "", false},
		{"host and port", "localhost:1883", "localhost", "1883", false},
		{"invalid multiple colons", "localhost:1883:9999", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port, err := splitHostPort(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("splitHostPort(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if host != tt.wantHost || port != tt.wantPort {
					t.Errorf("splitHostPort(%q) = (%q, %q), want (%q, %q)", tt.input, host, port, tt.wantHost, tt.wantPort)
				}
			}
		})
	}
}

func TestValidator(t *testing.T) {
	v := &Validator{}

	v.addError("test.field", "value", "message", "hint")
	if !v.hasErrors() {
		t.Error("Expected validator to have errors")
	}

	errs := v.getErrors()
	if len(errs) != 1 {
		t.Errorf("Expected 1 error, got %d", len(errs))
	}
}

// Helper function to check if message contains expected strings
func expects(msg string, fields ...string) bool {
	for _, field := range fields {
		if !contains(msg, field) {
			return false
		}
	}
	return true
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
