package input

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/optech/protocol-bridge/pkg/message"
)

// BACnetInput is a scaffold for BACnet/IP input (COV + polling fallback).
type BACnetInput struct {
	name     string
	logger   *zap.Logger
	messages chan *message.Message
}

func NewBACnetInput(name string, logger *zap.Logger) *BACnetInput {
	return &BACnetInput{name: name, logger: logger, messages: make(chan *message.Message, 100)}
}

func (b *BACnetInput) Start(ctx context.Context) error {
	b.logger.Info("BACnet input started", zap.String("name", b.name))
	// Implementation will include SubscribeCOV and RPM fallback.
	return nil
}

func (b *BACnetInput) Stop() error {
	close(b.messages)
	b.logger.Info("BACnet input stopped", zap.String("name", b.name))
	return nil
}

func (b *BACnetInput) Messages() <-chan *message.Message { return b.messages }
func (b *BACnetInput) Name() string                      { return b.name }
func (b *BACnetInput) IsConnected() bool                 { return true }

// emit is a helper to send a message downstream.
func (b *BACnetInput) emit(topic string, payload []byte) {
	msg := message.NewMessage(topic, payload, b.name)
	msg.SetMetadata("source_protocol", "bacnet")
	msg.SetMetadata("emitted_at", time.Now().UTC().Format(time.RFC3339Nano))
	select {
	case b.messages <- msg:
	default:
		b.logger.Warn("BACnet input channel full, dropping message")
	}
}
