package message

import (
	"encoding/json"
	"time"
)

// Message represents a message flowing through the pipeline
type Message struct {
	ID         string                 `json:"id"`
	Topic      string                 `json:"topic"`
	Payload    json.RawMessage        `json:"payload"`
	Headers    map[string]string      `json:"headers,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	Source     string                 `json:"source"`
	RetryCount int                    `json:"retry_count,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

// NewMessage creates a new message with the given payload and metadata
func NewMessage(topic string, payload json.RawMessage, source string) *Message {
	return &Message{
		ID:        generateID(),
		Topic:     topic,
		Payload:   payload,
		Headers:   make(map[string]string),
		Metadata:  make(map[string]interface{}),
		Timestamp: time.Now(),
		Source:    source,
	}
}

// SetHeader sets a header value
func (m *Message) SetHeader(key, value string) {
	if m.Headers == nil {
		m.Headers = make(map[string]string)
	}
	m.Headers[key] = value
}

// GetHeader gets a header value
func (m *Message) GetHeader(key string) string {
	if m.Headers == nil {
		return ""
	}
	return m.Headers[key]
}

// SetMetadata sets a metadata value
func (m *Message) SetMetadata(key string, value interface{}) {
	if m.Metadata == nil {
		m.Metadata = make(map[string]interface{})
	}
	m.Metadata[key] = value
}

// GetMetadata gets a metadata value
func (m *Message) GetMetadata(key string) interface{} {
	if m.Metadata == nil {
		return nil
	}
	return m.Metadata[key]
}

// IncrementRetry increments the retry count
func (m *Message) IncrementRetry() {
	m.RetryCount++
}

// SetError sets the error message
func (m *Message) SetError(err string) {
	m.Error = err
}

// ToJSON converts the message to JSON bytes
func (m *Message) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

// FromJSON creates a message from JSON bytes
func FromJSON(data []byte) (*Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	return &msg, err
}
