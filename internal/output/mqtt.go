package output

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"

	"github.com/optech/protocol-bridge/internal/config"
	"github.com/optech/protocol-bridge/internal/security"
	"github.com/optech/protocol-bridge/pkg/message"
)

// MQTTOutput implements the Output interface for MQTT brokers
type MQTTOutput struct {
	name    string
	config  config.OutputConfig
	client  mqtt.Client
	logger  *zap.Logger
	mu      sync.RWMutex
	started bool
	startMu sync.Mutex
}

// NewMQTTOutput creates a new MQTT output
func NewMQTTOutput(cfg config.OutputConfig, logger *zap.Logger) *MQTTOutput {
	return &MQTTOutput{
		name:   cfg.Name,
		config: cfg,
		logger: logger,
	}
}

// Start initializes the MQTT connection (idempotent - safe to call multiple times)
func (m *MQTTOutput) Start(ctx context.Context) error {
	m.startMu.Lock()
	defer m.startMu.Unlock()

	// If already started, return early
	if m.started {
		m.logger.Debug("MQTT output already started, skipping",
			zap.String("name", m.name))
		return nil
	}

	opts := mqtt.NewClientOptions()

	// Add broker (with or without tls:// prefix)
	brokerURL := m.config.Broker
	opts.AddBroker(brokerURL)

	opts.SetClientID(m.config.ClientID)
	opts.SetUsername(m.config.Username)
	opts.SetPassword(m.config.Password)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetMaxReconnectInterval(30 * time.Second)
	opts.SetConnectTimeout(30 * time.Second)
	opts.SetKeepAlive(60 * time.Second)
	opts.SetPingTimeout(10 * time.Second)

	// Configure TLS if certificates are provided
	if m.config.CertPath != "" && m.config.KeyPath != "" && m.config.CAPath != "" {
		// Check certificate expiration before creating TLS config
		_, err := security.CheckCertificateExpiration(m.config.CertPath, m.logger)
		if err != nil {
			m.logger.Error("failed to check certificate expiration", zap.Error(err))
		}

		tlsConfig, err := m.createTLSConfig()
		if err != nil {
			return fmt.Errorf("failed to create TLS config: %w", err)
		}
		opts.SetTLSConfig(tlsConfig)
		m.logger.Info("configured TLS for MQTT connection",
			zap.String("broker", brokerURL),
			zap.String("cert", m.config.CertPath),
			zap.String("key", m.config.KeyPath),
			zap.String("ca", m.config.CAPath))
	}

	// Set clean session to false for better reliability (matches working code)
	opts.SetCleanSession(false)

	// Connection handlers
	opts.SetOnConnectHandler(m.onConnect)
	opts.SetConnectionLostHandler(m.onConnectionLost)

	// Add reconnecting handler for better logging
	opts.SetReconnectingHandler(func(client mqtt.Client, opts *mqtt.ClientOptions) {
		m.logger.Info("attempting to reconnect to MQTT broker", zap.String("broker", m.config.Broker))
	})

	m.client = mqtt.NewClient(opts)

	// Start connection in background (non-blocking)
	go func() {
		m.logger.Info("attempting MQTT connection",
			zap.String("broker", m.config.Broker))

		token := m.client.Connect()

		// Wait for the connection to complete
		token.Wait()

		if token.Error() != nil {
			m.logger.Warn("MQTT connection failed",
				zap.String("broker", m.config.Broker),
				zap.Error(token.Error()))
		} else {
			// Verify actual connection status
			if m.client.IsConnected() {
				m.logger.Info("MQTT connection successful",
					zap.String("broker", m.config.Broker))
			} else {
				m.logger.Warn("MQTT connect returned no error but client not connected",
					zap.String("broker", m.config.Broker))
			}
		}
	}()

	m.started = true
	return nil
}

// Stop closes the MQTT connection
func (m *MQTTOutput) Stop() error {
	m.startMu.Lock()
	defer m.startMu.Unlock()

	if m.client != nil && m.client.IsConnected() {
		m.client.Disconnect(250)
	}
	m.started = false
	return nil
}

// Send sends a message to the MQTT broker.
// If the payload contains a "_topic" field it is used as the publish topic
// (and stripped from the payload), otherwise m.config.Topic is used.
func (m *MQTTOutput) Send(ctx context.Context, msg *message.Message) error {
	if !m.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	if msg.Payload == nil {
		return fmt.Errorf("message payload is empty")
	}

	// Resolve publish topic: prefer _topic field embedded in payload
	publishTopic := m.config.Topic
	payload := []byte(msg.Payload)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err == nil {
		if topicRaw, ok := raw["_topic"]; ok {
			var t string
			if err := json.Unmarshal(topicRaw, &t); err == nil && t != "" {
				publishTopic = t
				delete(raw, "_topic")
				if stripped, err := json.Marshal(raw); err == nil {
					payload = stripped
				}
			}
		}
	}

	if publishTopic == "" {
		return fmt.Errorf("no publish topic: set output topic in config or include _topic in payload")
	}

	token := m.client.Publish(
		publishTopic,
		byte(m.config.QoS),
		m.config.Retain,
		payload,
	)

	// Wait for completion with context timeout
	done := make(chan error, 1)
	go func() {
		done <- token.Error()
	}()

	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("failed to publish message: %w", err)
		}
		m.logger.Debug("message published",
			zap.String("topic", publishTopic),
			zap.Int("qos", m.config.QoS),
			zap.Bool("retain", m.config.Retain))
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Name returns the output name
func (m *MQTTOutput) Name() string {
	return m.name
}

// IsConnected returns the connection status (robust - uses actual client state)
func (m *MQTTOutput) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.client != nil && m.client.IsConnected()
}

// onConnect handles MQTT connection events
func (m *MQTTOutput) onConnect(client mqtt.Client) {
	m.logger.Info("connected to MQTT broker", zap.String("broker", m.config.Broker))
}

// onConnectionLost handles MQTT disconnection events
func (m *MQTTOutput) onConnectionLost(client mqtt.Client, err error) {
	m.logger.Error("lost connection to MQTT broker",
		zap.String("broker", m.config.Broker),
		zap.Error(err))
}

// createTLSConfig creates a TLS configuration from the provided certificate files
func (m *MQTTOutput) createTLSConfig() (*tls.Config, error) {
	// Load CA certificate FIRST
	caCert, err := os.ReadFile(m.config.CAPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	// Load client certificate
	cert, err := tls.LoadX509KeyPair(m.config.CertPath, m.config.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificate: %w", err)
	}

	// Create TLS configuration
	tlsConfig := &tls.Config{
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{cert},
		ServerName:   getServerNameFromBroker(m.config.Broker),
	}

	return tlsConfig, nil
}

// getServerNameFromBroker extracts the server name from the broker address
func getServerNameFromBroker(broker string) string {
	// Remove protocol prefix (tls:// or ssl://)
	broker = strings.TrimPrefix(broker, "tls://")
	broker = strings.TrimPrefix(broker, "ssl://")

	// Split by colon to get hostname and port
	parts := strings.Split(broker, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return broker
}
