package server

import (
	"testing"

	"go.uber.org/zap"

	"github.com/optech/protocol-bridge/internal/config"
	"github.com/optech/protocol-bridge/internal/output"
	"github.com/optech/protocol-bridge/internal/server/modbus"
)

func TestServerManager(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger)

	// Test adding a server
	serverConfig := config.ServerConfig{
		Name:    "test-mqtt",
		Type:    "mqtt",
		Enabled: true,
		Config: map[string]interface{}{
			"broker": "localhost:1883",
		},
	}

	manager.AddServer(serverConfig)

	// Test getting the server
	server, exists := manager.GetServer("test-mqtt")
	if !exists {
		t.Error("Expected server to exist")
	}

	if server.config.Name != "test-mqtt" {
		t.Errorf("Expected server name to be 'test-mqtt', got '%s'", server.config.Name)
	}

	// Test server is not running initially
	if server.IsRunning() {
		t.Error("Expected server to not be running initially")
	}
}

func TestMQTTServerDetection(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	server := &Server{
		config: config.ServerConfig{
			Name: "test-mqtt",
			Type: "mqtt",
			Config: map[string]interface{}{
				"broker": "localhost:1883",
			},
		},
		logger: logger,
	}

	// Test MQTT server detection (this will likely fail if no MQTT server is running)
	isRunning := server.isMQTTServerRunning("localhost:1883")
	// We can't assert the result since it depends on whether MQTT server is running
	t.Logf("MQTT server running on localhost:1883: %v", isRunning)
}

func TestDockerAvailability(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	server := &Server{
		logger: logger,
	}

	// Test Docker availability
	isAvailable := server.isDockerAvailable()
	t.Logf("Docker available: %v", isAvailable)
}

func TestDockerComposeExists(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	server := &Server{
		logger: logger,
	}

	// Test if docker-compose.yml exists in mqtt/server directory
	exists := server.dockerComposeExists("mqtt/server")
	t.Logf("docker-compose.yml exists in mqtt/server: %v", exists)
}

func TestModbusServerConfig(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	manager := NewManager(logger)

	// Test adding a Modbus server
	serverConfig := config.ServerConfig{
		Name:    "test-modbus",
		Type:    "modbus",
		Enabled: true,
		Config: map[string]interface{}{
			"address":          ":502",
			"persistence_file": "test_modbus_registers.json",
		},
	}

	manager.AddServer(serverConfig)

	// Test getting the server
	server, exists := manager.GetServer("test-modbus")
	if !exists {
		t.Error("Expected Modbus server to exist")
	}

	if server.config.Name != "test-modbus" {
		t.Errorf("Expected server name to be 'test-modbus', got '%s'", server.config.Name)
	}

	if server.config.Type != "modbus" {
		t.Errorf("Expected server type to be 'modbus', got '%s'", server.config.Type)
	}

	// Test Modbus context is nil initially
	if server.modbusCtx != nil {
		t.Error("Expected Modbus context to be nil initially")
	}
}

func TestOutputModbusValidation(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	// Test Modbus output validation
	modbusOutputConfig := config.OutputConfig{
		Name:   "modbus-test-output",
		Type:   "modbus",
		Server: "test-modbus-server",
	}

	// Create Modbus server context for testing
	modbusCtx := modbus.NewServerContext("test_registers.json", logger)
	
	// Test creating Modbus output
	output := output.NewModbusOutput(modbusOutputConfig.Name, modbusCtx, logger)
	
	if output.Name() != modbusOutputConfig.Name {
		t.Errorf("Expected output name to be '%s', got '%s'", modbusOutputConfig.Name, output.Name())
	}

	// Test IsConnected should return true if context exists
	if !output.IsConnected() {
		t.Error("Expected Modbus output to be connected")
	}
}
