package input

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/optech/protocol-bridge/internal/config"
)

func TestIntervalInput_EmitsOneMessagePerTopicPerTick(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := config.InputConfig{
		Name:   "test-interval",
		Type:   "interval",
		Topics: []string{"topic/a", "topic/b"},
	}
	in := NewIntervalInput(cfg, logger)
	in.intervalOverride = 20 * time.Millisecond // real config default (15s) would be too slow for a test

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := in.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer in.Stop()

	seen := map[string]int{}
	timeout := time.After(500 * time.Millisecond)
	for len(seen) < 2 || seen["topic/a"] == 0 || seen["topic/b"] == 0 {
		select {
		case msg := <-in.Messages():
			seen[msg.Topic]++
			if msg.Source != "test-interval" {
				t.Errorf("message Source = %q, want %q", msg.Source, "test-interval")
			}
		case <-timeout:
			t.Fatalf("timed out waiting for messages on both topics, saw: %v", seen)
		}
	}
}

func TestIntervalInput_NameAndConnectionState(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := config.InputConfig{Name: "my-ticker", Type: "interval", Topics: []string{"t"}}
	in := NewIntervalInput(cfg, logger)

	if in.Name() != "my-ticker" {
		t.Errorf("Name() = %q, want %q", in.Name(), "my-ticker")
	}
	if in.IsConnected() {
		t.Error("IsConnected() = true before Start(), want false")
	}

	in.intervalOverride = time.Hour // don't actually tick during this test
	ctx, cancel := context.WithCancel(context.Background())
	if err := in.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if !in.IsConnected() {
		t.Error("IsConnected() = false after Start(), want true")
	}

	cancel()
	if err := in.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
	if in.IsConnected() {
		t.Error("IsConnected() = true after Stop(), want false")
	}
}

func TestIntervalInput_DefaultIntervalWhenUnset(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	cfg := config.InputConfig{Name: "defaulted", Type: "interval", Topics: []string{"t"}}
	in := NewIntervalInput(cfg, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := in.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer in.Stop()

	if in.effectiveInterval() != 15*time.Second {
		t.Errorf("effectiveInterval() = %v, want 15s default when interval_seconds is unset/zero", in.effectiveInterval())
	}
}
