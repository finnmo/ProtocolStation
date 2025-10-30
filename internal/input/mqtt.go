package input

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"

	"github.com/optech/protocol-bridge/internal/config"
	"github.com/optech/protocol-bridge/internal/security"
	"github.com/optech/protocol-bridge/pkg/message"
)

// MQTTInput implements the Input interface for MQTT brokers
type MQTTInput struct {
	name      string
	config    config.InputConfig
	client    mqtt.Client
	messages  chan *message.Message
	logger    *zap.Logger
	connected bool
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewMQTTInput creates a new MQTT input
func NewMQTTInput(cfg config.InputConfig, logger *zap.Logger) *MQTTInput {
	return &MQTTInput{
		name:     cfg.Name,
		config:   cfg,
		messages: make(chan *message.Message, 100),
		logger:   logger,
	}
}

// Start begins consuming messages from the MQTT broker
func (m *MQTTInput) Start(ctx context.Context) error {
	m.ctx, m.cancel = context.WithCancel(ctx)

	opts := mqtt.NewClientOptions()
	opts.AddBroker(m.config.Broker)
	opts.SetClientID(m.config.ClientID)
	opts.SetUsername(m.config.Username)
	opts.SetPassword(m.config.Password)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetMaxReconnectInterval(30 * time.Second)
	opts.SetConnectTimeout(10 * time.Second)
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
			zap.String("broker", m.config.Broker),
			zap.String("cert", m.config.CertPath))
	}

	// Connection handlers
	opts.SetOnConnectHandler(m.onConnect)
	opts.SetConnectionLostHandler(m.onConnectionLost)

	m.client = mqtt.NewClient(opts)

	if token := m.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	// Subscribe to topics
	for _, topic := range m.config.Topics {
		if token := m.client.Subscribe(topic, byte(m.config.QoS), m.messageHandler); token.Wait() && token.Error() != nil {
			m.logger.Error("failed to subscribe to topic",
				zap.String("topic", topic),
				zap.Error(token.Error()))
			return fmt.Errorf("failed to subscribe to topic %s: %w", topic, token.Error())
		}
		m.logger.Info("subscribed to topic", zap.String("topic", topic))
	}

	return nil
}

// Stop stops consuming messages
func (m *MQTTInput) Stop() error {
	if m.cancel != nil {
		m.cancel()
	}

	if m.client != nil && m.client.IsConnected() {
		// Unsubscribe from all topics
		for _, topic := range m.config.Topics {
			if token := m.client.Unsubscribe(topic); token.Wait() && token.Error() != nil {
				m.logger.Error("failed to unsubscribe from topic",
					zap.String("topic", topic),
					zap.Error(token.Error()))
			}
		}
		m.client.Disconnect(250)
	}

	close(m.messages)
	return nil
}

// Messages returns the message channel
func (m *MQTTInput) Messages() <-chan *message.Message {
	return m.messages
}

// Name returns the input name
func (m *MQTTInput) Name() string {
	return m.name
}

// IsConnected returns the connection status
func (m *MQTTInput) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected && m.client != nil && m.client.IsConnected()
}

// onConnect handles MQTT connection events
// CRITICAL ensures subscriptions are restored on reconnect

func (m *MQTTInput) onConnect(client mqtt.Client) {
	m.mu.Lock()
	m.connected = true
	m.mu.Unlock()
	m.logger.Info("connected to MQTT broker", zap.String("broker", m.config.Broker))

	// Resubscribe to all topics after reconnection
	// This is critical because MQTT subscriptions are lost on disconnect
	for _, topic := range m.config.Topics {
		if token := client.Subscribe(topic, byte(m.config.QoS), m.messageHandler); token.Wait() && token.Error() != nil {
			m.logger.Error("failed to resubscribe to topic after reconnection",
				zap.String("topic", topic),
				zap.Error(token.Error()))
		} else {
			m.logger.Info("resubscribed to topic after reconnection", zap.String("topic", topic))
		}
	}
}

// onConnectionLost handles MQTT disconnection events
func (m *MQTTInput) onConnectionLost(client mqtt.Client, err error) {
	m.mu.Lock()
	m.connected = false
	m.mu.Unlock()
	m.logger.Error("lost connection to MQTT broker",
		zap.String("broker", m.config.Broker),
		zap.Error(err))
}

// messageHandler handles incoming MQTT messages
func (m *MQTTInput) messageHandler(client mqtt.Client, msg mqtt.Message) {
	select {
	case <-m.ctx.Done():
		return
	default:
	}

	m.logger.Debug("message received from broker",
		zap.String("topic", msg.Topic()),
		zap.Int("qos", int(msg.Qos())),
		zap.Int("payload_size", len(msg.Payload())))

	// Parse the payload as JSON
	var payload json.RawMessage
	if err := json.Unmarshal(msg.Payload(), &payload); err != nil {
		// If it's not valid JSON, wrap it as a string
		payload = json.RawMessage(fmt.Sprintf(`"%s"`, string(msg.Payload())))
	}

	// Create message
	message := message.NewMessage(msg.Topic(), payload, m.name)
	message.SetHeader("mqtt_qos", fmt.Sprintf("%d", msg.Qos()))
	message.SetHeader("mqtt_retained", fmt.Sprintf("%t", msg.Retained()))

	// Send to channel
	select {
	case m.messages <- message:

	case <-m.ctx.Done():
		return
	default:
		m.logger.Warn("message channel full, dropping message",
			zap.String("topic", msg.Topic()))
	}
}

// createTLSConfig creates a TLS configuration from the provided certificate files
func (m *MQTTInput) createTLSConfig() (*tls.Config, error) {
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
	}

	return tlsConfig, nil
}
