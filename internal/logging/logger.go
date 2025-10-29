package logging

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// LoggingConfig extends the base config with rotation settings
type LoggingConfig struct {
	Level      string          `yaml:"level"`
	Format     string          `yaml:"format"`
	File       string          `yaml:"file"`
	MaxSize    int             `yaml:"max_size"`    // Max size in MB
	MaxAge     int             `yaml:"max_age"`     // Max age in days
	MaxBackups int             `yaml:"max_backups"` // Max number of backups
	Compress   bool            `yaml:"compress"`    // Compress old log files
	Sampling   *SamplingConfig `yaml:"sampling"`
}

// SamplingConfig configures log sampling for high-volume logs
type SamplingConfig struct {
	Enabled    bool          `yaml:"enabled"`
	Tick       time.Duration `yaml:"tick"`       // Time between samples
	Initial    int           `yaml:"initial"`    // Initial entries to log
	Thereafter int           `yaml:"thereafter"` // Log 1 in every N after initial
}

// New creates a new structured logger with the given configuration
func New(config *LoggingConfig) (*zap.Logger, error) {
	// Parse log level
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(config.Level)); err != nil {
		level = zapcore.InfoLevel // Default to info
	}

	// Create encoder config
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	encoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder

	var encoder zapcore.Encoder
	if config.Format == "console" {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	// Create write syncer
	var writeSyncer zapcore.WriteSyncer
	if config.File != "" {
		// Use file with rotation
		writeSyncer = zapcore.AddSync(&lumberjack.Logger{
			Filename:   config.File,
			MaxSize:    config.MaxSize, // MB
			MaxBackups: config.MaxBackups,
			MaxAge:     config.MaxAge, // days
			Compress:   config.Compress,
			LocalTime:  true,
		})
	} else {
		// Use stdout/stderr
		writeSyncer = zapcore.AddSync(os.Stdout)
	}

	// Add sampling if enabled
	if config.Sampling != nil && config.Sampling.Enabled {
		writeSyncer = zapcore.AddSync(&sampledWriter{
			ws:         writeSyncer,
			tick:       config.Sampling.Tick,
			initial:    config.Sampling.Initial,
			thereafter: config.Sampling.Thereafter,
		})
	}

	// Create core
	core := zapcore.NewCore(encoder, writeSyncer, level)

	// Add caller and stack trace options
	opts := []zap.Option{
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	}

	logger := zap.New(core, opts...)
	return logger, nil
}

// sampledWriter implements log sampling to reduce high-volume log entries
type sampledWriter struct {
	ws         zapcore.WriteSyncer
	tick       time.Duration
	initial    int
	thereafter int
	lastReset  time.Time
	count      int
}

func (s *sampledWriter) Write(p []byte) (n int, err error) {
	now := time.Now()

	// Reset count if tick period has passed
	if now.Sub(s.lastReset) > s.tick {
		s.lastReset = now
		s.count = 0
	}

	// Check if we should log this entry
	s.count++
	if s.count <= s.initial || s.count%s.thereafter == 0 {
		return s.ws.Write(p)
	}

	// Skip this entry
	return len(p), nil
}

// Sync flushes any buffered log entries
func (s *sampledWriter) Sync() error {
	return s.ws.Sync()
}

// WithRequestID adds a request ID to the logger context
func WithRequestID(logger *zap.Logger, requestID string) *zap.Logger {
	return logger.With(zap.String("request_id", requestID))
}

// NewRequestID generates a unique request ID
func NewRequestID() string {
	// Simple implementation - in production, use crypto/rand
	now := time.Now()
	return now.Format("20060102150405") + "-" + string(rune(time.Now().UnixNano()))
}

