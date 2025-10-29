package output

import (
	"context"

	"github.com/optech/protocol-bridge/pkg/message"
)

// Output represents an output destination interface
type Output interface {
	// Send sends a message to the output destination
	Send(ctx context.Context, msg *message.Message) error

	// Name returns the output name
	Name() string

	// IsConnected returns the connection status
	IsConnected() bool

	// Start initializes the output connection
	Start(ctx context.Context) error

	// Stop closes the output connection
	Stop() error
}

// Factory creates output instances
type Factory interface {
	Create(config interface{}) (Output, error)
}
