package retry

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"

	"github.com/optech/protocol-bridge/internal/config"
	"github.com/optech/protocol-bridge/internal/output"
	"github.com/optech/protocol-bridge/pkg/message"
)

// DLQManager handles dead letter queue operations
type DLQManager struct {
	config         config.DLQConfig
	output         output.Output
	logger         *zap.Logger
	mu             sync.Mutex // protects messageCount and totalBytes
	messageCount   int64
	totalBytes     int64
	alertThreshold int64
}

// NewDLQManager creates a new DLQ manager
func NewDLQManager(cfg config.DLQConfig, logger *zap.Logger) *DLQManager {
	maxSize := int64(cfg.MaxSize) * 1024 * 1024 // Convert MB to bytes
	alertSize := int64(cfg.AlertSize) * 1024 * 1024
	if maxSize == 0 {
		maxSize = 100 * 1024 * 1024 // Default: 100MB
	}
	if alertSize == 0 {
		alertSize = 50 * 1024 * 1024 // Default: 50MB (alert at 50% of max)
	}

	return &DLQManager{
		config:         cfg,
		logger:         logger,
		alertThreshold: alertSize,
	}
}

// Initialize sets up the DLQ output
func (d *DLQManager) Initialize(ctx context.Context) error {
	if !d.config.Enabled {
		return nil
	}

	// Create MQTT output for DLQ
	if d.config.Type == "mqtt" {
		clientID := d.config.ClientID
		if clientID == "" {
			clientID = "protocol-bridge-dlq"
		}

		outputConfig := config.OutputConfig{
			Name:     "dlq-output",
			Type:     "mqtt",
			Broker:   d.config.Broker,
			ClientID: clientID,
			Topic:    d.config.Topic,
			QoS:      1,
			Retain:   false,
		}

		d.output = output.NewMQTTOutput(outputConfig, d.logger)
		return d.output.Start(ctx)
	}

	return fmt.Errorf("unsupported DLQ type: %s", d.config.Type)
}

// SendToDLQ sends a failed message to the dead letter queue
func (d *DLQManager) SendToDLQ(ctx context.Context, msg *message.Message, err error) error {
	if !d.config.Enabled {
		d.logger.Warn("DLQ not enabled, dropping failed message",
			zap.String("message_id", msg.ID),
			zap.Error(err))
		return nil
	}

	if d.output == nil {
		return fmt.Errorf("DLQ output not initialized")
	}

	// Estimate message size
	msgJSON, _ := msg.ToJSON()
	msgSize := int64(len(msgJSON))

	// Check and update DLQ size limits under lock
	d.mu.Lock()
	d.totalBytes += msgSize
	exceeded := d.totalBytes > int64(d.config.MaxSize)*1024*1024
	needAlert := d.totalBytes > d.alertThreshold && d.totalBytes-msgSize <= d.alertThreshold
	totalBytes := d.totalBytes
	d.mu.Unlock()

	if exceeded {
		d.logger.Error("DLQ size limit exceeded",
			zap.Int64("total_bytes", totalBytes),
			zap.Int64("limit", int64(d.config.MaxSize)*1024*1024),
			zap.Int64("message_count", d.messageCount))
		return fmt.Errorf("DLQ size limit exceeded: %d bytes", totalBytes)
	}

	if needAlert {
		d.logger.Warn("DLQ approaching size limit",
			zap.Int64("total_bytes", totalBytes),
			zap.Int64("alert_threshold", d.alertThreshold),
			zap.Int("max_size_mb", d.config.MaxSize))
	}

	// Add error information to message metadata
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]interface{})
	}
	msg.Metadata["dlq_error"] = err.Error()
	msg.Metadata["dlq_timestamp"] = msg.Timestamp
	msg.SetError(err.Error())

	// Send to DLQ
	dlqErr := d.output.Send(ctx, msg)
	if dlqErr != nil {
		d.logger.Error("failed to send message to DLQ",
			zap.String("message_id", msg.ID),
			zap.Error(dlqErr))
		return dlqErr
	}

	d.mu.Lock()
	d.messageCount++
	logCount := d.messageCount
	d.mu.Unlock()

	// Only log every 10th message to avoid log spam
	if logCount%10 == 0 {
		d.logger.Info("message sent to DLQ",
			zap.String("message_id", msg.ID),
			zap.String("topic", msg.Topic),
			zap.Int64("dlq_count", logCount),
			zap.Int64("dlq_bytes", totalBytes),
			zap.Error(err))
	}

	return nil
}

// Stop closes the DLQ output
func (d *DLQManager) Stop() error {
	if d.output != nil {
		return d.output.Stop()
	}
	return nil
}
