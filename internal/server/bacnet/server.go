package bacnet

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Server represents a minimal BACnet/IP server scaffold.
type Server struct {
	addr            string
	deviceInstances []uint32
	conn            *net.UDPConn
	logger          *zap.Logger
	mu              sync.RWMutex
	running         bool
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewServer creates a new BACnet/IP server.
func NewServer(address string, deviceInstances []uint32, logger *zap.Logger) *Server {
	return &Server{addr: address, deviceInstances: deviceInstances, logger: logger}
}

// Start begins listening on UDP/47808 (or configured port) and logs basic BACnet frames.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return fmt.Errorf("BACnet server already running")
	}

	udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return fmt.Errorf("resolve UDP addr failed: %w", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("listen UDP failed: %w", err)
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.conn = conn
	s.running = true

	s.logger.Info("BACnet/IP server started",
		zap.String("address", s.addr),
		zap.Int("devices", len(s.deviceInstances)))

	go s.readLoop()
	return nil
}

// Stop stops the server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return nil
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.conn != nil {
		s.conn.Close()
	}
	s.running = false
	s.logger.Info("BACnet/IP server stopped")
	return nil
}

func (s *Server) readLoop() {
	buf := make([]byte, 1500)
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		s.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, remote, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			// timeout or closed; continue until stop
			continue
		}
		// Minimal log of incoming BVLL/NPUD/APDU frame size
		s.logger.Info("BACnet packet received",
			zap.String("remote", remote.String()),
			zap.Int("bytes", n))

		// NOTE: Proper decoding and replies (Who-Is/I-Am, RP/RPM/WP) will be implemented
		// in subsequent iterations using a BACnet library for standards compliance.
	}
}

// HealthCheck performs a simple reachability check.
func (s *Server) HealthCheck() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.running {
		return fmt.Errorf("BACnet server not running")
	}
	return nil
}

