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
		zap.String("name", m.name))
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
					Liters int32 `json:"liters"`
				} `json:"transformedPayload"`
			} `json:"body"`
		} `json:"message"`
	}

	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("failed to parse message payload: %w", err)
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

	// Parse register number (e.g., "403021" -> 3021)
	registerNum, err := strconv.Atoi(modbusRegisterStr[1:]) // Skip first character '4'
	if err != nil {
		return fmt.Errorf("invalid modbus register number: %s", modbusRegisterStr)
	}

	// Get liters value
	liters := payload.Message.Body.TransformedPayload.Liters

	// Update the Modbus server's holding register
	// The register number is already the offset (e.g., 3021)
	m.serverCtx.UpdateRegister(unitID, registerNum, liters)

	m.logger.Info("Updated Modbus register from message",
		zap.String("deviceID", payload.Message.Header.DeviceID),
		zap.Int("unitID", unitID),
		zap.Int("register", registerNum),
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
