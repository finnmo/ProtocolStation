package output

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"

	"github.com/optech/protocol-bridge/internal/config"
	"github.com/optech/protocol-bridge/pkg/message"
)

// fakeToken simulates an async paho Token whose Error() only becomes valid
// after Done() closes - exactly like the real library. Send() must wait on
// Done() before reading Error(), or it reports false success/failure.
type fakeToken struct {
	done chan struct{}
	err  error
}

// newDelayedToken resolves (closing done, then setting err) only after
// delay, to catch any code path that reads Error() before completion.
func newDelayedToken(delay time.Duration, err error) *fakeToken {
	tok := &fakeToken{done: make(chan struct{})}
	go func() {
		time.Sleep(delay)
		tok.err = err
		close(tok.done)
	}()
	return tok
}

func (f *fakeToken) Wait() bool { <-f.done; return true }
func (f *fakeToken) WaitTimeout(d time.Duration) bool {
	select {
	case <-f.done:
		return true
	case <-time.After(d):
		return false
	}
}
func (f *fakeToken) Done() <-chan struct{} { return f.done }
func (f *fakeToken) Error() error          { return f.err }

// fakeClient is a minimal mqtt.Client fake - only Publish/IsConnected are
// meaningfully implemented since that's all Send() exercises.
type fakeClient struct {
	connected    bool
	publishToken mqtt.Token
	lastTopic    string
	lastPayload  []byte
}

func (f *fakeClient) IsConnected() bool       { return f.connected }
func (f *fakeClient) IsConnectionOpen() bool  { return f.connected }
func (f *fakeClient) Connect() mqtt.Token     { return nil }
func (f *fakeClient) Disconnect(quiesce uint) {}
func (f *fakeClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	f.lastTopic = topic
	if b, ok := payload.([]byte); ok {
		f.lastPayload = b
	}
	return f.publishToken
}
func (f *fakeClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
	return nil
}
func (f *fakeClient) SubscribeMultiple(filters map[string]byte, callback mqtt.MessageHandler) mqtt.Token {
	return nil
}
func (f *fakeClient) Unsubscribe(topics ...string) mqtt.Token             { return nil }
func (f *fakeClient) AddRoute(topic string, callback mqtt.MessageHandler) {}
func (f *fakeClient) OptionsReader() mqtt.ClientOptionsReader             { return mqtt.ClientOptionsReader{} }

func newTestOutput(t *testing.T, client mqtt.Client) *MQTTOutput {
	t.Helper()
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	return &MQTTOutput{
		name:    t.Name(),
		config:  config.OutputConfig{Name: t.Name(), Topic: "default/topic", QoS: 1},
		client:  client,
		logger:  logger,
		started: true,
	}
}

// TestMQTTOutputSend_WaitsForTokenCompletion is a regression test: Send()
// must not report success (or failure) before the publish token actually
// completes. Token.Error() just reads a field with no blocking, so reading
// it immediately after Publish() - without waiting on Done() first - would
// observe the token's zero-value nil error regardless of the real outcome,
// which resolves only after newDelayedToken's goroutine runs.
func TestMQTTOutputSend_WaitsForTokenCompletion(t *testing.T) {
	wantErr := errors.New("broker rejected publish")
	client := &fakeClient{
		connected:    true,
		publishToken: newDelayedToken(20*time.Millisecond, wantErr),
	}
	out := newTestOutput(t, client)

	msg := &message.Message{ID: "1", Payload: json.RawMessage(`{"a":1}`)}
	err := out.Send(context.Background(), msg)

	if err == nil {
		t.Fatalf("Send() = nil, want error wrapping %q - the token resolved to a real error after a delay, so success here means Error() was read before Done() closed", wantErr)
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("Send() error = %v, want wrapping %v", err, wantErr)
	}
}

func TestMQTTOutputSend_SuccessAfterDelay(t *testing.T) {
	client := &fakeClient{
		connected:    true,
		publishToken: newDelayedToken(20*time.Millisecond, nil),
	}
	out := newTestOutput(t, client)

	msg := &message.Message{ID: "1", Payload: json.RawMessage(`{"a":1}`)}
	if err := out.Send(context.Background(), msg); err != nil {
		t.Errorf("Send() = %v, want nil", err)
	}
	if client.lastTopic != "default/topic" {
		t.Errorf("published to %q, want %q", client.lastTopic, "default/topic")
	}
}

func TestMQTTOutputSend_ContextCancelledBeforeTokenCompletes(t *testing.T) {
	client := &fakeClient{
		connected:    true,
		publishToken: newDelayedToken(time.Hour, nil), // never resolves within the test
	}
	out := newTestOutput(t, client)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	msg := &message.Message{ID: "1", Payload: json.RawMessage(`{"a":1}`)}
	err := out.Send(ctx, msg)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Send() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestMQTTOutputSend_NotConnected(t *testing.T) {
	client := &fakeClient{connected: false}
	out := newTestOutput(t, client)

	msg := &message.Message{ID: "1", Payload: json.RawMessage(`{"a":1}`)}
	if err := out.Send(context.Background(), msg); err == nil {
		t.Error("Send() = nil, want error when client is not connected")
	}
}

func TestMQTTOutputSend_TopicOverrideFromPayload(t *testing.T) {
	client := &fakeClient{
		connected:    true,
		publishToken: newDelayedToken(time.Millisecond, nil),
	}
	out := newTestOutput(t, client)

	msg := &message.Message{ID: "1", Payload: json.RawMessage(`{"_topic":"overridden/topic","a":1}`)}
	if err := out.Send(context.Background(), msg); err != nil {
		t.Fatalf("Send() = %v, want nil", err)
	}
	if client.lastTopic != "overridden/topic" {
		t.Errorf("published to %q, want %q", client.lastTopic, "overridden/topic")
	}
	if string(client.lastPayload) == `{"_topic":"overridden/topic","a":1}` {
		t.Error("_topic field should have been stripped from the published payload")
	}
}
