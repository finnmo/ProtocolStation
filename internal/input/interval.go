package input

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/optech/protocol-bridge/internal/config"
	"github.com/optech/protocol-bridge/pkg/message"
)

// IntervalInput periodically emits one synthetic message per configured
// topic, on a fixed interval, regardless of any external activity.
// Transformers only run on message arrival, so this is how time-based
// logic (e.g. a heartbeat when no real event has arrived recently) gets
// driven into an otherwise purely reactive pipeline.
type IntervalInput struct {
	name      string
	config    config.InputConfig
	logger    *zap.Logger
	messages  chan *message.Message
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	connected bool
	mu        sync.RWMutex

	// intervalOverride takes precedence over config.IntervalSeconds when
	// set - exists only so tests don't have to wait out a real interval.
	intervalOverride time.Duration
}

// effectiveInterval resolves the configured interval, falling back to a
// 15s default when unset, or to intervalOverride when set (tests only).
func (i *IntervalInput) effectiveInterval() time.Duration {
	if i.intervalOverride > 0 {
		return i.intervalOverride
	}
	interval := time.Duration(i.config.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 15 * time.Second
	}
	return interval
}

// NewIntervalInput creates a new interval input
func NewIntervalInput(cfg config.InputConfig, logger *zap.Logger) *IntervalInput {
	return &IntervalInput{
		name:     cfg.Name,
		config:   cfg,
		logger:   logger,
		messages: make(chan *message.Message, 10),
	}
}

// Start begins emitting messages on the configured interval
func (i *IntervalInput) Start(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	i.cancel = cancel

	interval := i.effectiveInterval()

	i.mu.Lock()
	i.connected = true
	i.mu.Unlock()

	i.wg.Add(1)
	go func() {
		defer i.wg.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, topic := range i.config.Topics {
					msg := message.NewMessage(topic, json.RawMessage("{}"), i.name)
					select {
					case i.messages <- msg:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	i.logger.Info("interval input started",
		zap.String("name", i.name),
		zap.Duration("interval", interval),
		zap.Strings("topics", i.config.Topics))
	return nil
}

// Stop stops emitting messages
func (i *IntervalInput) Stop() error {
	if i.cancel != nil {
		i.cancel()
	}
	i.wg.Wait()

	i.mu.Lock()
	i.connected = false
	i.mu.Unlock()

	i.logger.Info("interval input stopped", zap.String("name", i.name))
	return nil
}

// Messages returns the channel for receiving messages
func (i *IntervalInput) Messages() <-chan *message.Message {
	return i.messages
}

// Name returns the input name
func (i *IntervalInput) Name() string {
	return i.name
}

// IsConnected returns whether the ticker is running
func (i *IntervalInput) IsConnected() bool {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.connected
}
