package output

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"go.uber.org/zap"

	"github.com/optech/protocol-bridge/internal/server/modbus"
	"github.com/optech/protocol-bridge/pkg/message"
)

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ModbusOutput writes messages to a Modbus server's holding registers
type ModbusOutput struct {
	name      string
	serverCtx *modbus.ServerContext
	logger    *zap.Logger
}

// NewModbusOutput creates a new Modbus output
func NewModbusOutput(name string, serverCtx *modbus.ServerContext, logger *zap.Logger) *ModbusOutput {
	return &ModbusOutput{
		name:      name,
		serverCtx: serverCtx,
		logger:    logger,
	}
}

// Start initializes the output
func (m *ModbusOutput) Start(ctx context.Context) error {
	m.logger.Info("Modbus output started",
		zap.String("name", m.name),
		zap.String("address", m.serverCtx.Address()))
	return nil
}

// Stop stops the output
func (m *ModbusOutput) Stop() error {
	m.logger.Info("Modbus output stopped",
		zap.String("name", m.name))
	return nil
}

// Send sends a message to the Modbus server
func (m *ModbusOutput) Send(ctx context.Context, msg *message.Message) error {
	// Parse the payload to extract deviceID, unitID, modbusRegister, and liters
	var payload struct {
		Message struct {
			Header struct {
				DeviceID string `json:"deviceID"`
			} `json:"header"`
			Body struct {
				Attributes struct {
					ModbusRegister string `json:"modbusRegister"`
					UnitID         string `json:"unitID"`
				} `json:"attributes"`
				TransformedPayload struct {
					Liters *int32 `json:"liters"` // Use pointer to detect missing/null values
				} `json:"transformedPayload"`
			} `json:"body"`
		} `json:"message"`
	}

	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		m.logger.Debug("Failed to parse message payload, logging raw payload for debugging",
			zap.String("raw_payload", string(msg.Payload)),
			zap.Error(err))
		return fmt.Errorf("failed to parse message payload: %w", err)
	}

	// Log the parsed liters value for debugging (especially for unit 21)
	if payload.Message.Body.Attributes.UnitID == "21" {
		m.logger.Debug("Parsed MQTT message for unit 21",
			zap.String("deviceID", payload.Message.Header.DeviceID),
			zap.String("unitID", payload.Message.Body.Attributes.UnitID),
			zap.String("modbusRegister", payload.Message.Body.Attributes.ModbusRegister),
			zap.Any("liters", payload.Message.Body.TransformedPayload.Liters),
			zap.String("raw_payload_preview", string(msg.Payload)[:min(200, len(msg.Payload))]))
	}

	// Extract unitID - handle empty string
	if payload.Message.Body.Attributes.UnitID == "" {
		// Silently drop messages without unitID instead of failing
		m.logger.Debug("empty unitID in message, dropping silently",
			zap.String("deviceID", payload.Message.Header.DeviceID))
		return nil // Return nil to indicate successful "drop" and avoid retries
	}

	unitID, err := strconv.Atoi(payload.Message.Body.Attributes.UnitID)
	if err != nil {
		return fmt.Errorf("invalid unitID: %s", payload.Message.Body.Attributes.UnitID)
	}

	// Extract modbusRegister and convert to offset
	modbusRegisterStr := payload.Message.Body.Attributes.ModbusRegister
	if modbusRegisterStr == "" || len(modbusRegisterStr) < 6 {
		// Silently drop messages with missing or invalid modbus register
		m.logger.Debug("empty or invalid modbus register in message, dropping silently",
			zap.String("deviceID", payload.Message.Header.DeviceID),
			zap.String("modbusRegister", modbusRegisterStr))
		return nil // Return nil to indicate successful "drop" and avoid retries
	}

	// Compute server register offset to match legacy behavior:
	// offset = serverRegister - 400001. Example: 403021 -> 3020
	// Accept both string like "403021" and numeric-like strings.
	serverReg, err := strconv.Atoi(modbusRegisterStr)
	if err != nil {
		return fmt.Errorf("invalid modbus register number: %s", modbusRegisterStr)
	}
	registerNum := serverReg - 400001
	if registerNum < 0 {
		m.logger.Debug("computed negative register offset, dropping message",
			zap.String("deviceID", payload.Message.Header.DeviceID),
			zap.Int("unitID", unitID),
			zap.Int("serverRegister", serverReg))
		return nil
	}

	// Get liters value - check if it's present (not missing/null)
	if payload.Message.Body.TransformedPayload.Liters == nil {
		m.logger.Error("Dropping message with missing/null liters value",
			zap.String("deviceID", payload.Message.Header.DeviceID),
			zap.Int("unitID", unitID),
			zap.Int("register_offset", registerNum),
			zap.String("raw_payload_preview", string(msg.Payload)[:min(500, len(msg.Payload))]))
		// Drop the message to avoid overwriting valid register values with missing data
		return nil
	}

	liters := *payload.Message.Body.TransformedPayload.Liters

	// Note: We allow 0 values as they are valid readings (e.g., meter reset or actual zero consumption)
	// Only missing/null values are rejected above

	// Update the Modbus server's holding register
	m.serverCtx.UpdateRegister(unitID, registerNum, liters)

	m.logger.Info("Updated Modbus register from message",
		zap.String("deviceID", payload.Message.Header.DeviceID),
		zap.Int("unitID", unitID),
		zap.Int("register_offset", registerNum),
		zap.Int("server_register", serverReg),
		zap.Int32("liters", liters))

	return nil
}

// Name returns the output name
func (m *ModbusOutput) Name() string {
	return m.name
}

// IsConnected returns the connection status
func (m *ModbusOutput) IsConnected() bool {
	// Modbus output is always connected if server context exists
	return m.serverCtx != nil
}
