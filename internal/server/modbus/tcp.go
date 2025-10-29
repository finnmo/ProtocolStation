package modbus

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"

	"go.uber.org/zap"
)

// TCPServer represents a Modbus TCP server
type TCPServer struct {
	address     string
	serverCtx   *ServerContext
	logger      *zap.Logger
	listener    net.Listener
	connections sync.WaitGroup
	running     bool
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewTCPServer creates a new Modbus TCP server
func NewTCPServer(address string, serverCtx *ServerContext, logger *zap.Logger) *TCPServer {
	return &TCPServer{
		address:   address,
		serverCtx: serverCtx,
		logger:    logger,
	}
}

// Start starts the Modbus TCP server
func (s *TCPServer) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("Modbus server is already running")
	}

	s.ctx, s.cancel = context.WithCancel(ctx)

	// Load saved values
	if err := s.serverCtx.LoadSavedValues(); err != nil {
		s.logger.Error("Failed to load saved values", zap.Error(err))
		// Continue anyway
	}

	ln, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("failed to start Modbus TCP server: %w", err)
	}

	s.listener = ln
	s.running = true

	s.logger.Info("Modbus TCP server started",
		zap.String("address", s.address))

	// Accept connections
	go s.acceptConnections()

	return nil
}

// Stop stops the Modbus TCP server
func (s *TCPServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.running = false

	if s.cancel != nil {
		s.cancel()
	}

	if s.listener != nil {
		s.listener.Close()
	}

	// Wait for all connections to close
	s.connections.Wait()

	s.logger.Info("Modbus TCP server stopped")
	return nil
}

// acceptConnections accepts incoming TCP connections
func (s *TCPServer) acceptConnections() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			if s.running {
				s.logger.Error("Error accepting connection", zap.Error(err))
			}
			return
		}

		s.connections.Add(1)
		go func(c net.Conn) {
			defer s.connections.Done()
			s.handleConnection(c)
		}(conn)
	}
}

// handleConnection handles a Modbus TCP connection
func (s *TCPServer) handleConnection(c net.Conn) {
	defer func() {
		if err := recover(); err != nil {
			s.logger.Error("Panic in connection handler",
				zap.Any("error", err))
		}
		c.Close()
	}()

	s.logger.Debug("Accepted Modbus connection",
		zap.String("remote", c.RemoteAddr().String()))

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Read MBAP header (7 bytes)
		header := make([]byte, 7)
		if _, err := io.ReadFull(c, header); err != nil {
			if err != io.EOF && s.running {
				s.logger.Debug("Error reading MBAP header",
					zap.Error(err))
			}
			return
		}

		transactionID := binary.BigEndian.Uint16(header[0:2])
		protocolID := binary.BigEndian.Uint16(header[2:4])
		length := binary.BigEndian.Uint16(header[4:6])
		unitID := int(header[6])

		if protocolID != 0 {
			s.logger.Warn("Unexpected protocol ID",
				zap.Uint16("protocolID", protocolID))
			return
		}

		// Read PDU
		pduLength := int(length) - 1
		pdu := make([]byte, pduLength)
		if _, err := io.ReadFull(c, pdu); err != nil {
			s.logger.Debug("Error reading PDU", zap.Error(err))
			return
		}

		if len(pdu) < 5 {
			s.logger.Warn("PDU too short", zap.Int("length", len(pdu)))
			continue
		}

		functionCode := pdu[0]

		// Handle function codes
		switch functionCode {
		case 3: // Read Holding Registers
			s.handleReadHoldingRegisters(c, transactionID, unitID, pdu)
		default:
			s.logger.Debug("Unsupported function code",
				zap.Uint8("functionCode", functionCode))
			s.sendExceptionResponse(c, transactionID, byte(unitID), functionCode, 0x01)
		}
	}
}

// handleReadHoldingRegisters handles function code 3 (Read Holding Registers)
func (s *TCPServer) handleReadHoldingRegisters(c net.Conn, transactionID uint16, unitID int, pdu []byte) {
	startAddress := binary.BigEndian.Uint16(pdu[1:3])
	quantity := binary.BigEndian.Uint16(pdu[3:5])

	s.logger.Debug("Read Holding Registers request",
		zap.Int("unitID", unitID),
		zap.Uint16("startAddress", startAddress),
		zap.Uint16("quantity", quantity))

	slave, ok := s.serverCtx.Slaves[unitID]
	if !ok {
		s.logger.Warn("Unit not found",
			zap.Int("unitID", unitID))
		s.sendExceptionResponse(c, transactionID, byte(unitID), 0x03, 0x0B)
		return
	}

	start := int(startAddress)
	reqQty := int(quantity)

	slave.Mu.RLock()
	if start+reqQty > len(slave.HR) {
		s.logger.Warn("Requested registers out of range",
			zap.Int("start", start),
			zap.Int("quantity", reqQty))
		slave.Mu.RUnlock()
		s.sendExceptionResponse(c, transactionID, byte(unitID), 0x03, 0x02)
		return
	}
	registers := make([]uint16, reqQty)
	copy(registers, slave.HR[start:start+reqQty])
	slave.Mu.RUnlock()

	// Build response
	byteCount := uint8(len(registers) * 2)
	responsePDU := make([]byte, 2+len(registers)*2)
	responsePDU[0] = 0x03
	responsePDU[1] = byteCount

	for i, reg := range registers {
		binary.BigEndian.PutUint16(responsePDU[2+i*2:2+i*2+2], reg)
	}

	// Build MBAP header
	responseLength := uint16(len(responsePDU) + 1)
	responseHeader := make([]byte, 7)
	binary.BigEndian.PutUint16(responseHeader[0:2], transactionID)
	binary.BigEndian.PutUint16(responseHeader[2:4], 0)
	binary.BigEndian.PutUint16(responseHeader[4:6], responseLength)
	responseHeader[6] = byte(unitID)

	response := append(responseHeader, responsePDU...)
	if _, err := c.Write(response); err != nil {
		s.logger.Error("Error writing response", zap.Error(err))
		return
	}

	s.logger.Debug("Sent Read Holding Registers response",
		zap.Uint16("transactionID", transactionID))
}

// sendExceptionResponse sends a Modbus exception response
func (s *TCPServer) sendExceptionResponse(c net.Conn, transactionID uint16, unitID byte, functionCode byte, exceptionCode byte) {
	exceptionFunctionCode := functionCode | 0x80
	pdu := []byte{exceptionFunctionCode, exceptionCode}
	responseLength := uint16(len(pdu) + 1)
	header := make([]byte, 7)
	binary.BigEndian.PutUint16(header[0:2], transactionID)
	binary.BigEndian.PutUint16(header[2:4], 0)
	binary.BigEndian.PutUint16(header[4:6], responseLength)
	header[6] = unitID

	response := append(header, pdu...)
	if _, err := c.Write(response); err != nil {
		s.logger.Error("Error writing exception response", zap.Error(err))
	}
}
