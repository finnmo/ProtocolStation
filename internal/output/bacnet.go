package output

import (
	"context"

	"go.uber.org/zap"

	"github.com/optech/protocol-bridge/pkg/message"
)

// BACnetOutput scaffolds WriteProperty mapping to a hosted BACnet server.
type BACnetOutput struct {
	name   string
	server string
	logger *zap.Logger
}

func NewBACnetOutput(name, server string, logger *zap.Logger) *BACnetOutput {
	return &BACnetOutput{name: name, server: server, logger: logger}
}

func (b *BACnetOutput) Start(ctx context.Context) error {
	b.logger.Info("BACnet output started", zap.String("name", b.name), zap.String("server", b.server))
	return nil
}

func (b *BACnetOutput) Stop() error {
	b.logger.Info("BACnet output stopped", zap.String("name", b.name))
	return nil
}

func (b *BACnetOutput) Send(ctx context.Context, msg *message.Message) error {
	// Future: map msg to WriteProperty against hosted server objects
	b.logger.Debug("BACnet output send", zap.String("name", b.name), zap.String("message_id", msg.ID))
	return nil
}

func (b *BACnetOutput) Name() string      { return b.name }
func (b *BACnetOutput) IsConnected() bool { return true }

