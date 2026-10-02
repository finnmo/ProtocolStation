package config

import (
	"fmt"
	"os"
	"time"

	"github.com/optech/protocol-bridge/internal/security"
	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	Servers       []ServerConfig      `yaml:"servers"`
	Inputs        []InputConfig       `yaml:"inputs"`
	Pipelines     []PipelineConfig    `yaml:"pipelines"`
	Transformers  []TransformerConfig `yaml:"transformers"`
	Outputs       []OutputConfig      `yaml:"outputs"`
	ErrorHandling ErrorHandlingConfig `yaml:"error_handling"`
	Logging       LoggingConfig       `yaml:"logging"`
}

// ServerConfig represents a hosted server configuration
type ServerConfig struct {
	Name    string                 `yaml:"name"`
	Type    string                 `yaml:"type"`
	Enabled bool                   `yaml:"enabled"`
	Config  map[string]interface{} `yaml:",inline"`
}

// InputConfig represents an input source configuration
type InputConfig struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Broker   string   `yaml:"broker"`
	ClientID string   `yaml:"client_id"`
	Topics   []string `yaml:"topics"`
	QoS      int      `yaml:"qos"`
	Username string   `yaml:"username"`
	Password string   `yaml:"password"`
	// TLS/Certificate fields
	CertPath string `yaml:"cert_path"`
	KeyPath  string `yaml:"key_path"`
	CAPath   string `yaml:"ca_path"`
	// Interval-type fields: emits one message per entry in Topics, every
	// IntervalSeconds, regardless of external activity.
	IntervalSeconds int `yaml:"interval_seconds"`
}

// PipelineConfig represents a message pipeline configuration
type PipelineConfig struct {
	Name   string          `yaml:"name"`
	Input  string          `yaml:"input"`
	Routes []PipelineRoute `yaml:"routes"`
}

// PipelineRoute represents a route within a pipeline
type PipelineRoute struct {
	Transformer string   `yaml:"transformer"`
	Outputs     []string `yaml:"outputs"`
}

// TransformerConfig represents a transformer configuration
type TransformerConfig struct {
	Name   string `yaml:"name"`
	Type   string `yaml:"type"`
	Script string `yaml:"script"`
}

// OutputConfig represents an output destination configuration
type OutputConfig struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
	// MQTT-specific
	Broker   string `yaml:"broker"`
	ClientID string `yaml:"client_id"`
	Topic    string `yaml:"topic"`
	QoS      int    `yaml:"qos"`
	Retain   bool   `yaml:"retain"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	// TLS/Certificate fields
	CertPath string `yaml:"cert_path"`
	KeyPath  string `yaml:"key_path"`
	CAPath   string `yaml:"ca_path"`
	// Modbus-specific
	Server string `yaml:"server"`
}

// ErrorHandlingConfig represents error handling configuration
type ErrorHandlingConfig struct {
	Retry RetryConfig `yaml:"retry"`
	DLQ   DLQConfig   `yaml:"dead_letter_queue"`
}

// RetryConfig represents retry configuration
type RetryConfig struct {
	Enabled         bool          `yaml:"enabled"`
	MaxAttempts     int           `yaml:"max_attempts"`
	InitialInterval time.Duration `yaml:"initial_interval"`
	MaxInterval     time.Duration `yaml:"max_interval"`
	Multiplier      float64       `yaml:"multiplier"`
}

// DLQConfig represents dead letter queue configuration
type DLQConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Type      string `yaml:"type"`
	Broker    string `yaml:"broker"`
	Topic     string `yaml:"topic"`
	ClientID  string `yaml:"client_id"`
	MaxSize   int    `yaml:"max_size"`   // Maximum DLQ size in MB before alerting
	AlertSize int    `yaml:"alert_size"` // Size in MB to trigger alert
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	File       string `yaml:"file"`
	MaxSize    int    `yaml:"max_size"`
	MaxAge     int    `yaml:"max_age"`
	MaxBackups int    `yaml:"max_backups"`
	Compress   bool   `yaml:"compress"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Decrypt sensitive fields
	if err := decryptSensitiveFields(&config); err != nil {
		return nil, fmt.Errorf("failed to decrypt sensitive fields: %w", err)
	}

	// Set defaults
	setDefaults(&config)

	// Validate configuration
	err = validateConfig(&config)
	if err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// setDefaults sets default values for configuration
func setDefaults(config *Config) {
	// Set default retry configuration
	if config.ErrorHandling.Retry.MaxAttempts == 0 {
		config.ErrorHandling.Retry.MaxAttempts = 3
	}
	if config.ErrorHandling.Retry.InitialInterval == 0 {
		config.ErrorHandling.Retry.InitialInterval = time.Second
	}
	if config.ErrorHandling.Retry.MaxInterval == 0 {
		config.ErrorHandling.Retry.MaxInterval = 30 * time.Second
	}
	if config.ErrorHandling.Retry.Multiplier == 0 {
		config.ErrorHandling.Retry.Multiplier = 2.0
	}

	// Set default logging configuration
	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	if config.Logging.Format == "" {
		config.Logging.Format = "json"
	}
	if config.Logging.File == "" {
		config.Logging.File = "logs/bridge.log"
	}
	if config.Logging.MaxSize == 0 {
		config.Logging.MaxSize = 100 // MB
	}
	if config.Logging.MaxAge == 0 {
		config.Logging.MaxAge = 7 // days
	}
	if config.Logging.MaxBackups == 0 {
		config.Logging.MaxBackups = 5
	}
	if !config.Logging.Compress {
		config.Logging.Compress = true
	}
}

// validateConfig is now implemented in validation.go with comprehensive checks

// decryptSensitiveFields decrypts encrypted sensitive fields in the configuration.
// It only decrypts fields that start with the encryption prefix.
func decryptSensitiveFields(config *Config) error {
	// Decrypt input passwords
	for i := range config.Inputs {
		if config.Inputs[i].Password != "" {
			decrypted, err := security.DecryptConfigValue(config.Inputs[i].Password)
			if err != nil {
				return fmt.Errorf("failed to decrypt password for input %s: %w", config.Inputs[i].Name, err)
			}
			config.Inputs[i].Password = decrypted
		}
	}

	// Decrypt output passwords
	for i := range config.Outputs {
		if config.Outputs[i].Password != "" {
			decrypted, err := security.DecryptConfigValue(config.Outputs[i].Password)
			if err != nil {
				return fmt.Errorf("failed to decrypt password for output %s: %w", config.Outputs[i].Name, err)
			}
			config.Outputs[i].Password = decrypted
		}
	}

	return nil
}
