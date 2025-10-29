package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/optech/protocol-bridge/internal/config"
	"github.com/optech/protocol-bridge/internal/server/modbus"
)

// Manager manages hosted servers
type Manager struct {
	servers           map[string]*Server
	logger            *zap.Logger
	mu                sync.RWMutex
	healthCheckTicker *time.Ticker
	healthCheckCtx    context.Context
	healthCheckCancel context.CancelFunc
}

// Server represents a hosted server
type Server struct {
	config    config.ServerConfig
	cmd       *exec.Cmd
	logger    *zap.Logger
	running   bool
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	modbusTCP *modbus.TCPServer     // For Modbus TCP servers
	modbusCtx *modbus.ServerContext // Shared Modbus context
}

// NewManager creates a new server manager
func NewManager(logger *zap.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		servers:           make(map[string]*Server),
		logger:            logger,
		healthCheckCtx:    ctx,
		healthCheckCancel: cancel,
	}
}

// StartAll starts all enabled servers and begins health monitoring
func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, server := range m.servers {
		if server.config.Enabled {
			if err := server.Start(ctx); err != nil {
				m.logger.Error("failed to start server",
					zap.String("name", name),
					zap.Error(err))
				return fmt.Errorf("failed to start server %s: %w", name, err)
			}
			m.logger.Info("server started", zap.String("name", name))
		}
	}

	// Start periodic health monitoring
	m.startHealthMonitoring(ctx)

	return nil
}

// startHealthMonitoring starts periodic health checks every 60 seconds
func (m *Manager) startHealthMonitoring(ctx context.Context) {
	m.healthCheckTicker = time.NewTicker(60 * time.Second)

	go func() {
		for {
			select {
			case <-ctx.Done():
				m.healthCheckTicker.Stop()
				return
			case <-m.healthCheckTicker.C:
				m.performHealthChecks(ctx)
			}
		}
	}()

	m.logger.Info("server health monitoring started")
}

// performHealthChecks performs health checks on all servers and attempts auto-recovery
func (m *Manager) performHealthChecks(ctx context.Context) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, server := range m.servers {
		if !server.config.Enabled || server.config.Type != "mqtt" {
			continue
		}

		// Check server health
		if err := server.HealthCheck(); err != nil {
			m.logger.Warn("server health check failed",
				zap.String("name", name),
				zap.Error(err))

			// Attempt to restart the server
			if err := server.Restart(ctx); err != nil {
				m.logger.Error("failed to restart server",
					zap.String("name", name),
					zap.Error(err))
			} else {
				m.logger.Info("server restarted successfully",
					zap.String("name", name))
			}
		}
	}
}

// StopAll stops all servers and health monitoring
func (m *Manager) StopAll() error {
	// Stop health monitoring
	if m.healthCheckCancel != nil {
		m.healthCheckCancel()
	}
	if m.healthCheckTicker != nil {
		m.healthCheckTicker.Stop()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for name, server := range m.servers {
		if err := server.Stop(); err != nil {
			m.logger.Error("failed to stop server",
				zap.String("name", name),
				zap.Error(err))
		} else {
			m.logger.Info("server stopped", zap.String("name", name))
		}
	}

	return nil
}

// AddServer adds a server configuration
func (m *Manager) AddServer(cfg config.ServerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	server := &Server{
		config: cfg,
		logger: m.logger.With(zap.String("server", cfg.Name)),
	}
	m.servers[cfg.Name] = server
}

// GetServer returns a server by name
func (m *Manager) GetServer(name string) (*Server, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	server, exists := m.servers[name]
	return server, exists
}

// GetModbusContext returns the Modbus server context from a server
func (s *Server) GetModbusContext() *modbus.ServerContext {
	return s.modbusCtx
}

// Start starts the server
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("server %s is already running", s.config.Name)
	}

	s.ctx, s.cancel = context.WithCancel(ctx)

	switch s.config.Type {
	case "mqtt":
		return s.startMQTTServer()
	case "modbus":
		return s.startModbusServer(ctx)
	default:
		return fmt.Errorf("unsupported server type: %s", s.config.Type)
	}
}

// Stop stops the server
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	if s.cancel != nil {
		s.cancel()
	}

	// Stop Docker Compose services if it's an MQTT server
	if s.config.Type == "mqtt" {
		if err := s.stopMQTTServer(); err != nil {
			s.logger.Error("failed to stop MQTT server", zap.Error(err))
		}
	}

	// Stop Modbus TCP server
	if s.config.Type == "modbus" && s.modbusTCP != nil {
		if err := s.modbusTCP.Stop(); err != nil {
			s.logger.Error("failed to stop Modbus server", zap.Error(err))
		}
	}

	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
		s.cmd.Wait()
	}

	s.running = false
	s.logger.Info("server stopped", zap.String("name", s.config.Name))
	return nil
}

// stopMQTTServer stops the MQTT server using Docker Compose
func (s *Server) stopMQTTServer() error {
	mqttDir := "mqtt/server"
	if !s.dockerComposeExists(mqttDir) {
		s.logger.Warn("docker-compose.yml not found, skipping MQTT server stop")
		return nil
	}

	s.logger.Info("stopping MQTT server using Docker Compose")

	cmd := exec.Command("docker-compose", "down")
	cmd.Dir = mqttDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		s.logger.Error("failed to stop MQTT server",
			zap.String("output", string(output)),
			zap.Error(err))
		return fmt.Errorf("failed to stop MQTT server: %w", err)
	}

	s.logger.Info("MQTT server stopped successfully")
	return nil
}

// IsRunning returns whether the server is running
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// startMQTTServer starts an MQTT server using Docker Compose
func (s *Server) startMQTTServer() error {
	broker, ok := s.config.Config["broker"].(string)
	if !ok {
		broker = "localhost:1883"
	}

	// Check if MQTT server is already running
	if s.isMQTTServerRunning(broker) {
		s.logger.Info("MQTT server already running", zap.String("broker", broker))
		s.running = true
		return nil
	}

	// Check if Docker is available
	if !s.isDockerAvailable() {
		return fmt.Errorf("Docker is not available - cannot start MQTT server")
	}

	// Check if docker-compose.yml exists in mqtt/server directory
	mqttDir := "mqtt/server"
	if !s.dockerComposeExists(mqttDir) {
		s.logger.Warn("docker-compose.yml not found in mqtt/server directory",
			zap.String("directory", mqttDir))
		return fmt.Errorf("MQTT server configuration not found")
	}

	// Start MQTT server using Docker Compose
	s.logger.Info("starting MQTT server using Docker Compose", zap.String("broker", broker))

	cmd := exec.CommandContext(s.ctx, "docker-compose", "up", "-d")
	cmd.Dir = mqttDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)

		// Check if it's a network-related error
		if s.isNetworkError(outputStr) {
			s.logger.Warn("encountered network conflict, attempting cleanup and retry",
				zap.String("output", outputStr))

			// Try to cleanup and retry
			if cleanupErr := s.cleanupMQTTServer(); cleanupErr != nil {
				s.logger.Warn("failed to cleanup MQTT server", zap.Error(cleanupErr))
			}

			// Retry once
			cmd := exec.CommandContext(s.ctx, "docker-compose", "up", "-d")
			cmd.Dir = mqttDir

			output, err = cmd.CombinedOutput()
			if err != nil {
				outputStr = string(output)
				s.logger.Error("failed to start MQTT server after retry",
					zap.String("output", outputStr),
					zap.Error(err))
				return fmt.Errorf("failed to start MQTT server after retry: %w", err)
			}

			s.logger.Info("MQTT server started successfully after cleanup")
		} else {
			s.logger.Error("failed to start MQTT server",
				zap.String("output", outputStr),
				zap.Error(err))
			return fmt.Errorf("failed to start MQTT server: %w", err)
		}
	}

	// Wait for server to be ready
	if err := s.waitForMQTTServer(broker, 30*time.Second); err != nil {
		s.logger.Error("MQTT server failed to become ready", zap.Error(err))
		return fmt.Errorf("MQTT server not ready: %w", err)
	}

	s.running = true
	s.logger.Info("MQTT server started successfully", zap.String("broker", broker))
	return nil
}

// startModbusServer starts a Modbus TCP server
func (s *Server) startModbusServer(ctx context.Context) error {
	address, ok := s.config.Config["address"].(string)
	if !ok {
		address = ":502"
	}

	persistenceFile := "modbus_registers.json"
	if pfile, ok := s.config.Config["persistence_file"].(string); ok {
		persistenceFile = pfile
	}

	// Create Modbus server context
	s.modbusCtx = modbus.NewServerContext(persistenceFile, s.logger)

	// Create and start Modbus TCP server
	s.modbusTCP = modbus.NewTCPServer(address, s.modbusCtx, s.logger)

	if err := s.modbusTCP.Start(ctx); err != nil {
		return fmt.Errorf("failed to start Modbus server: %w", err)
	}

	s.running = true
	s.logger.Info("Modbus server started successfully", zap.String("address", address))
	return nil
}

// HealthCheck performs a health check on the server
func (s *Server) HealthCheck() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.running {
		return fmt.Errorf("server %s is not running", s.config.Name)
	}

	// Implement actual health checks based on server type
	switch s.config.Type {
	case "mqtt":
		return s.checkMQTTHealth()
	case "modbus":
		return s.checkModbusHealth()
	default:
		return fmt.Errorf("unsupported server type for health check: %s", s.config.Type)
	}
}

// checkMQTTHealth checks MQTT server health
func (s *Server) checkMQTTHealth() error {
	broker, ok := s.config.Config["broker"].(string)
	if !ok {
		broker = "localhost:1883"
	}

	host, port, err := net.SplitHostPort(broker)
	if err != nil {
		return fmt.Errorf("invalid broker address: %w", err)
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 2*time.Second)
	if err != nil {
		return fmt.Errorf("MQTT server not reachable: %w", err)
	}
	conn.Close()
	return nil
}

// checkModbusHealth checks Modbus server health
func (s *Server) checkModbusHealth() error {
	address, ok := s.config.Config["address"].(string)
	if !ok {
		address = ":502"
	}

	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return fmt.Errorf("Modbus server not reachable: %w", err)
	}
	conn.Close()
	return nil
}

// isMQTTServerRunning checks if MQTT server is already running
func (s *Server) isMQTTServerRunning(broker string) bool {
	host, port, err := net.SplitHostPort(broker)
	if err != nil {
		return false
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// isDockerAvailable checks if Docker is available
func (s *Server) isDockerAvailable() bool {
	cmd := exec.Command("docker", "version")
	return cmd.Run() == nil
}

// dockerComposeExists checks if docker-compose.yml exists in the given directory
func (s *Server) dockerComposeExists(dir string) bool {
	composeFile := filepath.Join(dir, "docker-compose.yml")
	_, err := os.Stat(composeFile)
	return err == nil
}

// waitForMQTTServer waits for MQTT server to become ready
func (s *Server) waitForMQTTServer(broker string, timeout time.Duration) error {
	host, port, err := net.SplitHostPort(broker)
	if err != nil {
		return fmt.Errorf("invalid broker address: %w", err)
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		default:
		}

		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 1*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for MQTT server to become ready")
}

// isNetworkError checks if the error output indicates a network-related issue
func (s *Server) isNetworkError(output string) bool {
	networkErrors := []string{
		"network",
		"not found",
		"already exists",
		"duplicate",
		"conflict",
	}

	outputLower := strings.ToLower(output)
	for _, errMsg := range networkErrors {
		if strings.Contains(outputLower, errMsg) {
			return true
		}
	}
	return false
}

// cleanupMQTTServer attempts to clean up orphaned Docker networks and containers
func (s *Server) cleanupMQTTServer() error {
	mqttDir := "mqtt/server"
	s.logger.Info("attempting to cleanup MQTT Docker environment")

	// Try to stop any running containers
	cmd := exec.CommandContext(s.ctx, "docker-compose", "down")
	cmd.Dir = mqttDir

	if output, err := cmd.CombinedOutput(); err != nil {
		// Log but don't fail - cleanup might fail if nothing is running
		s.logger.Debug("docker-compose down completed",
			zap.String("output", string(output)),
			zap.Error(err))
	}

	// Wait a moment for cleanup to complete
	time.Sleep(2 * time.Second)

	return nil
}

// Restart restarts the server
func (s *Server) Restart(ctx context.Context) error {
	s.logger.Info("restarting server", zap.String("name", s.config.Name))

	if err := s.Stop(); err != nil {
		return fmt.Errorf("failed to stop server during restart: %w", err)
	}

	// Wait a bit before restarting
	time.Sleep(2 * time.Second)

	if err := s.Start(ctx); err != nil {
		return fmt.Errorf("failed to start server during restart: %w", err)
	}

	return nil
}
