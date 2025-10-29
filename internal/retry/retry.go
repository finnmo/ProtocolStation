package retry

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"go.uber.org/zap"

	"github.com/optech/protocol-bridge/internal/config"
	"github.com/optech/protocol-bridge/pkg/message"
)

// RetryManager handles retry logic with exponential backoff
type RetryManager struct {
	config config.RetryConfig
	logger *zap.Logger
}

// NewRetryManager creates a new retry manager
func NewRetryManager(cfg config.RetryConfig, logger *zap.Logger) *RetryManager {
	return &RetryManager{
		config: cfg,
		logger: logger,
	}
}

// ExecuteWithRetry executes a function with retry logic
func (r *RetryManager) ExecuteWithRetry(ctx context.Context, operation func() error, operationName string) error {
	if !r.config.Enabled {
		return operation()
	}

	var lastErr error
	attempt := 0

	// Create exponential backoff
	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.InitialInterval = r.config.InitialInterval
	backoffConfig.MaxInterval = r.config.MaxInterval
	backoffConfig.Multiplier = r.config.Multiplier
	backoffConfig.MaxElapsedTime = 0 // No max elapsed time, we control attempts

	for attempt < r.config.MaxAttempts {
		attempt++

		err := operation()
		if err == nil {
			if attempt > 1 {
				// Only log as debug to reduce log noise for transient startup retries
				r.logger.Debug("operation succeeded after retry",
					zap.String("operation", operationName),
					zap.Int("attempt", attempt))
			}
			return nil
		}

		lastErr = err

		if attempt < r.config.MaxAttempts {
			// Calculate delay for next attempt
			delay := backoffConfig.NextBackOff()

			r.logger.Warn("operation failed, retrying",
				zap.String("operation", operationName),
				zap.Int("attempt", attempt),
				zap.Int("max_attempts", r.config.MaxAttempts),
				zap.Duration("delay", delay),
				zap.Error(err))

			// Wait for delay or context cancellation
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}

	r.logger.Error("operation failed after all retries",
		zap.String("operation", operationName),
		zap.Int("attempts", attempt),
		zap.Error(lastErr))

	return fmt.Errorf("operation %s failed after %d attempts: %w", operationName, attempt, lastErr)
}

// ExecuteMessageWithRetry executes a message processing operation with retry logic
func (r *RetryManager) ExecuteMessageWithRetry(ctx context.Context, msg *message.Message, operation func(*message.Message) error, operationName string) error {
	return r.ExecuteWithRetry(ctx, func() error {
		// Increment retry count on the message
		msg.IncrementRetry()
		return operation(msg)
	}, fmt.Sprintf("%s (message %s)", operationName, msg.ID))
}
