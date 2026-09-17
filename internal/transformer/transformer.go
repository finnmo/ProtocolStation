package transformer

import (
	"context"

	"github.com/optech/protocol-bridge/pkg/message"
)

// Transformer represents a message transformer interface
type Transformer interface {
	// Transform processes a message and returns the transformed result
	Transform(ctx context.Context, msg *message.Message) (*message.Message, error)

	// Name returns the transformer name
	Name() string

	// Type returns the transformer type
	Type() string
}

// Factory creates transformer instances
type Factory interface {
	Create(config interface{}) (Transformer, error)
}
