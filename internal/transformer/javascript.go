package transformer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dop251/goja"
	"go.uber.org/zap"

	"github.com/optech/protocol-bridge/internal/config"
	"github.com/optech/protocol-bridge/pkg/message"
)

// JavaScriptTransformer implements the Transformer interface using JavaScript
type JavaScriptTransformer struct {
	name    string
	config  config.TransformerConfig
	vm      *goja.Runtime
	logger  *zap.Logger
	timeout time.Duration
}

// NewJavaScriptTransformer creates a new JavaScript transformer
func NewJavaScriptTransformer(cfg config.TransformerConfig, logger *zap.Logger) *JavaScriptTransformer {
	return &JavaScriptTransformer{
		name:    cfg.Name,
		config:  cfg,
		logger:  logger,
		timeout: 5 * time.Second,
	}
}

// Transform processes a message using JavaScript
func (j *JavaScriptTransformer) Transform(ctx context.Context, msg *message.Message) (*message.Message, error) {
	// Create a new VM for each transformation to ensure isolation
	vm := goja.New()

	// Set timeout for the transformation
	done := make(chan error, 1)
	var result *message.Message

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("JavaScript panic: %v", r)
			}
		}()

		// Parse the input JSON
		var inputData interface{}
		if err := json.Unmarshal(msg.Payload, &inputData); err != nil {
			done <- fmt.Errorf("failed to parse input JSON: %w", err)
			return
		}

		// Create context object with message metadata
		contextData := map[string]interface{}{
			"topic":     msg.Topic,
			"source":    msg.Source,
			"timestamp": msg.Timestamp,
		}

		// Set up the JavaScript environment
		vm.Set("input", inputData)
		vm.Set("context", contextData)
		vm.Set("console", map[string]interface{}{
			"log": func(args ...interface{}) {
				j.logger.Debug("JS console.log", zap.Any("args", args))
			},
		})

		// Execute the transformation script
		vm.RunString(j.config.Script)

		// Call the transform function
		transformFn, ok := goja.AssertFunction(vm.Get("transform"))
		if !ok {
			done <- fmt.Errorf("transform function not found in script")
			return
		}

		// Execute the transform function with both input and context
		jsResult, err := transformFn(goja.Undefined(), vm.Get("input"), vm.Get("context"))
		if err != nil {
			done <- fmt.Errorf("failed to execute transform function: %w", err)
			return
		}

		// Convert result to Go value
		goResult := jsResult.Export()

		// Check if result is null/undefined (filtered out)
		if goResult == nil {
			result = nil
			done <- nil
			return
		}

		// Convert to JSON
		resultJSON, err := json.Marshal(goResult)
		if err != nil {
			done <- fmt.Errorf("failed to marshal result to JSON: %w", err)
			return
		}

		// Create new message with transformed payload
		result = &message.Message{
			ID:        msg.ID,
			Topic:     msg.Topic,
			Payload:   json.RawMessage(resultJSON),
			Headers:   msg.Headers,
			Metadata:  msg.Metadata,
			Timestamp: msg.Timestamp,
			Source:    msg.Source,
		}

		// Copy metadata and add transformation info
		if result.Metadata == nil {
			result.Metadata = make(map[string]interface{})
		}
		result.Metadata["transformer"] = j.name
		result.Metadata["transformed_at"] = time.Now()

		done <- nil
	}()

	// Wait for completion or timeout
	select {
	case err := <-done:
		if err != nil {
			return nil, err
		}
		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(j.timeout):
		return nil, fmt.Errorf("transformation timeout after %v", j.timeout)
	}
}

// Name returns the transformer name
func (j *JavaScriptTransformer) Name() string {
	return j.name
}

// Type returns the transformer type
func (j *JavaScriptTransformer) Type() string {
	return "javascript"
}
