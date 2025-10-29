package config

import (
	"testing"
	"time"
)

func TestConfigDefaults(t *testing.T) {
	config := &Config{
		ErrorHandling: ErrorHandlingConfig{},
		Logging:       LoggingConfig{},
	}

	setDefaults(config)

	// Test retry defaults
	if config.ErrorHandling.Retry.MaxAttempts != 3 {
		t.Errorf("Expected MaxAttempts to be 3, got %d", config.ErrorHandling.Retry.MaxAttempts)
	}

	if config.ErrorHandling.Retry.InitialInterval != time.Second {
		t.Errorf("Expected InitialInterval to be 1s, got %v", config.ErrorHandling.Retry.InitialInterval)
	}

	if config.ErrorHandling.Retry.MaxInterval != 30*time.Second {
		t.Errorf("Expected MaxInterval to be 30s, got %v", config.ErrorHandling.Retry.MaxInterval)
	}

	if config.ErrorHandling.Retry.Multiplier != 2.0 {
		t.Errorf("Expected Multiplier to be 2.0, got %f", config.ErrorHandling.Retry.Multiplier)
	}

	// Test logging defaults
	if config.Logging.Level != "info" {
		t.Errorf("Expected Logging.Level to be 'info', got '%s'", config.Logging.Level)
	}

	if config.Logging.Format != "json" {
		t.Errorf("Expected Logging.Format to be 'json', got '%s'", config.Logging.Format)
	}
}

func TestConfigValidation(t *testing.T) {
	config := &Config{
		Inputs: []InputConfig{
			{Name: "test-input", Type: "mqtt", Broker: "localhost:1883", Topics: []string{"test/topic"}, QoS: 1, ClientID: "client-1"},
		},
		Transformers: []TransformerConfig{
			{Name: "test-transformer", Type: "javascript", Script: "function transform(input) { return input; }"},
		},
		Outputs: []OutputConfig{
			{Name: "test-output", Type: "mqtt", Broker: "localhost:1883", Topic: "output/topic", QoS: 1, ClientID: "client-2"},
		},
		Pipelines: []PipelineConfig{
			{
				Name:  "test-pipeline",
				Input: "test-input",
				Routes: []PipelineRoute{
					{
						Transformer: "test-transformer",
						Outputs:     []string{"test-output"},
					},
				},
			},
		},
	}

	err := validateConfig(config)
	if err != nil {
		t.Errorf("Expected validation to pass, got error: %v", err)
	}
}

func TestConfigValidationErrors(t *testing.T) {
	config := &Config{
		Inputs: []InputConfig{
			{Name: "test-input", Type: "mqtt"},
		},
		Pipelines: []PipelineConfig{
			{
				Name:  "test-pipeline",
				Input: "unknown-input", // This should cause validation error
				Routes: []PipelineRoute{
					{
						Transformer: "unknown-transformer",
						Outputs:     []string{"unknown-output"},
					},
				},
			},
		},
	}

	err := validateConfig(config)
	if err == nil {
		t.Error("Expected validation to fail, but it passed")
	}
}
