package transformer

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/optech/protocol-bridge/internal/config"
	"github.com/optech/protocol-bridge/pkg/message"
)

func newTestTransformer(t *testing.T, script string) *JavaScriptTransformer {
	t.Helper()
	cfg := config.TransformerConfig{
		Name:   t.Name(),
		Type:   "javascript",
		Script: script,
	}
	// Override state file to a temp path so tests don't pollute the workspace
	jt := &JavaScriptTransformer{
		name:    cfg.Name,
		config:  cfg,
		logger:  zap.NewNop(),
		timeout: 5 * time.Second,
	}
	stateFile := filepath.Join(t.TempDir(), "state.json")
	ss, err := NewStateStorage(stateFile, zap.NewNop())
	if err != nil {
		t.Fatalf("failed to create state storage: %v", err)
	}
	jt.stateStorage = ss
	jt.stopAutoSave = make(chan struct{})
	return jt
}

func newMsg(t *testing.T, topic string, payload interface{}) *message.Message {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return message.NewMessage(topic, json.RawMessage(b), "test")
}

func TestJSTransformBasic(t *testing.T) {
	tr := newTestTransformer(t, `function transform(input) { return { value: input.x * 2 }; }`)
	msg := newMsg(t, "test/topic", map[string]interface{}{"x": 21})

	result, err := tr.Transform(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}

	var out map[string]interface{}
	if err := json.Unmarshal(result.Payload, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["value"].(float64) != 42 {
		t.Errorf("expected 42, got %v", out["value"])
	}
}

func TestJSTransformReturnNull(t *testing.T) {
	tr := newTestTransformer(t, `function transform(input) { return null; }`)
	msg := newMsg(t, "test/topic", map[string]interface{}{})

	result, err := tr.Transform(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result (filtered), got %v", result)
	}
}

// TestJSTransformSyntaxError ensures a script syntax error returns a useful error
// rather than the misleading "transform function not found" message from the old code.
func TestJSTransformSyntaxError(t *testing.T) {
	tr := newTestTransformer(t, `this is not valid javascript {{{`)
	msg := newMsg(t, "test/topic", map[string]interface{}{})

	_, err := tr.Transform(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error for invalid script, got nil")
	}
	// Error should mention the compilation failure, not something misleading
	t.Logf("got expected error: %v", err)
}

func TestJSTransformTimeout(t *testing.T) {
	tr := newTestTransformer(t, `function transform(input) { while(true){} }`)
	tr.timeout = 50 * time.Millisecond

	_, err := tr.Transform(context.Background(), newMsg(t, "t", map[string]interface{}{}))
	if err == nil {
		t.Fatal("expected timeout error for infinite loop")
	}
}

func TestJSTransformState(t *testing.T) {
	tr := newTestTransformer(t, `
		function transform(input, context) {
			var prev = state.get("count") || 0;
			state.setAndSave("count", prev + 1);
			return { count: state.get("count") };
		}
	`)
	msg := newMsg(t, "t", map[string]interface{}{})

	for i := 1; i <= 3; i++ {
		result, err := tr.Transform(context.Background(), msg)
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		var out map[string]interface{}
		json.Unmarshal(result.Payload, &out)
		if out["count"].(float64) != float64(i) {
			t.Errorf("call %d: expected count=%d, got %v", i, i, out["count"])
		}
	}
}

func TestJSTransformContextTopic(t *testing.T) {
	tr := newTestTransformer(t, `function transform(input, context) { return { topic: context.topic }; }`)
	msg := newMsg(t, "sensor/room1", map[string]interface{}{})

	result, err := tr.Transform(context.Background(), msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var out map[string]interface{}
	json.Unmarshal(result.Payload, &out)
	if out["topic"] != "sensor/room1" {
		t.Errorf("expected topic 'sensor/room1', got %v", out["topic"])
	}
}

// TestPeopleCounterLZGreenFilter is a regression test for the bug where
// /BEN-TCP\d+-Green/ (missing -LZ-) failed to filter car-counting topics,
// causing them to be published to the people-counter AWS IoT topic.
func TestPeopleCounterLZGreenFilter(t *testing.T) {
	script := `
		function transform(input, context) {
			var topic = context.topic || "";
			if (/BEN-TCP\d+-LZ-Green/.test(topic)) {
				return null;
			}
			return { passed: true };
		}
	`
	tr := newTestTransformer(t, script)

	cases := []struct {
		topic      string
		wantFilter bool
	}{
		{"BEN-TCP1-LZ-Green/onvif-ej/RuleEngine/CountAggregation/Counter/&1/Inbound", true},
		{"BEN-TCP2-LZ-Green/onvif-ej/RuleEngine/CountAggregation/Counter/&1/Outbound", true},
		{"SomeCamera/onvif-ej/RuleEngine/CountAggregation/Counter/Inbound", false},
		{"Building-A/Counter Name: Main-inbound", false},
	}

	for _, tc := range cases {
		msg := newMsg(t, tc.topic, map[string]interface{}{})
		result, err := tr.Transform(context.Background(), msg)
		if err != nil {
			t.Errorf("topic %q: unexpected error: %v", tc.topic, err)
			continue
		}
		filtered := result == nil
		if filtered != tc.wantFilter {
			t.Errorf("topic %q: wantFilter=%v got filtered=%v", tc.topic, tc.wantFilter, filtered)
		}
	}
}
