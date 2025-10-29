package input

import (
	"context"

	"github.com/optech/protocol-bridge/pkg/message"
)

// Input represents an input source interface
type Input interface {
	// Start begins consuming messages from the input source
	Start(ctx context.Context) error

	// Stop stops consuming messages
	Stop() error

	// Messages returns a channel for receiving messages
	Messages() <-chan *message.Message

	// Name returns the input name
	Name() string

	// IsConnected returns the connection status
	IsConnected() bool
}

// Factory creates input instances
type Factory interface {
	Create(config interface{}) (Input, error)
}
