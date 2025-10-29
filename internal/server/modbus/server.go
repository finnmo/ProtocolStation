package modbus

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/optech/protocol-bridge/internal/health"
)

// RegisterValue represents a saved register value
type RegisterValue struct {
	UnitID         int   `json:"unitID"`
	RegisterOffset int   `json:"registerOffset"`
	Value          int32 `json:"value"`
	Timestamp      int64 `json:"timestamp"`
}

// SlaveContext holds the holding registers for a Modbus unit
type SlaveContext struct {
	HR []uint16
	Mu sync.RWMutex
}

// ServerContext holds all the slave contexts
type ServerContext struct {
	Slaves          map[int]*SlaveContext
	mu              sync.RWMutex
	persistenceFile string
	logger          *zap.Logger
}

// NewServerContext creates a new Modbus server context
func NewServerContext(persistenceFile string, logger *zap.Logger) *ServerContext {
	sc := &ServerContext{
		Slaves:          make(map[int]*SlaveContext),
		persistenceFile: persistenceFile,
		logger:          logger,
	}

	// Pre-allocate slave contexts for unit IDs 1 to 100
	for unit := 1; unit <= 100; unit++ {
		sc.Slaves[unit] = &SlaveContext{
			HR: make([]uint16, 5000),
		}
	}

	return sc
}

// LoadSavedValues loads previously saved register values from disk
func (sc *ServerContext) LoadSavedValues() error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if _, err := os.Stat(sc.persistenceFile); os.IsNotExist(err) {
		sc.logger.Info("No saved values found, starting with empty registers",
			zap.String("file", sc.persistenceFile))
		return nil
	}

	data, err := os.ReadFile(sc.persistenceFile)
	if err != nil {
		return fmt.Errorf("error reading persistence file: %w", err)
	}

	var values []RegisterValue
	if err := json.Unmarshal(data, &values); err != nil {
		return fmt.Errorf("error unmarshaling saved values: %w", err)
	}

	count := 0
	for _, v := range values {
		if v.Timestamp < time.Now().Add(-24*time.Hour).Unix() {
			sc.logger.Debug("Skipping old value",
				zap.Int("unitID", v.UnitID),
				zap.Int("offset", v.RegisterOffset))
			continue
		}

		slave, ok := sc.Slaves[v.UnitID]
		if !ok {
			sc.logger.Warn("Unit not found for saved value",
				zap.Int("unitID", v.UnitID))
			continue
		}

		// Convert int32 to two uint16 values (big endian)
		buf := make([]byte, 4)
		binary.BigEndian.PutUint32(buf, uint32(v.Value))
		r1 := binary.BigEndian.Uint16(buf[0:2])
		r2 := binary.BigEndian.Uint16(buf[2:4])

		slave.Mu.Lock()
		if v.RegisterOffset >= 0 && v.RegisterOffset+1 < len(slave.HR) {
			slave.HR[v.RegisterOffset] = r1
			slave.HR[v.RegisterOffset+1] = r2
			count++
		}
		slave.Mu.Unlock()
	}

	sc.logger.Info("Loaded saved register values",
		zap.Int("count", count))
	return nil
}

// SaveValue saves a register value to the persistence file
// This method now includes cleanup of old entries to prevent unbounded growth
func (sc *ServerContext) SaveValue(unitID int, registerOffset int, value int32) error {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	regValue := RegisterValue{
		UnitID:         unitID,
		RegisterOffset: registerOffset,
		Value:          value,
		Timestamp:      time.Now().Unix(),
	}

	// Read existing values
	var values []RegisterValue
	if data, err := os.ReadFile(sc.persistenceFile); err == nil {
		json.Unmarshal(data, &values)
	}

	// Cleanup: remove entries older than 30 days to prevent unbounded growth
	cutoffTime := time.Now().Add(-30 * 24 * time.Hour).Unix()
	cleanedValues := make([]RegisterValue, 0, len(values))
	for _, v := range values {
		if v.Timestamp >= cutoffTime {
			cleanedValues = append(cleanedValues, v)
		}
	}
	values = cleanedValues

	// Update or add the new value
	found := false
	for i, v := range values {
		if v.UnitID == unitID && v.RegisterOffset == registerOffset {
			values[i] = regValue
			found = true
			break
		}
	}
	if !found {
		values = append(values, regValue)
	}

	// Check file size before writing - alert if approaching 1GB limit
	const maxFileSize = 1000 * 1024 * 1024 // 1GB
	if info, err := os.Stat(sc.persistenceFile); err == nil {
		if info.Size() > maxFileSize {
			sc.logger.Warn("Modbus persistence file approaching size limit, consider compaction",
				zap.Int64("size", info.Size()),
				zap.Int("entries", len(values)))
		}
	}

	// Save to file
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling values: %w", err)
	}

	// Check disk space before creating directory and writing file
	if err := health.CheckDiskSpaceBeforeWrite(sc.persistenceFile, uint64(len(data)), sc.logger); err != nil {
		return fmt.Errorf("disk space check failed: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(sc.persistenceFile), 0755); err != nil {
		return fmt.Errorf("error creating directory: %w", err)
	}

	if err := os.WriteFile(sc.persistenceFile, data, 0644); err != nil {
		return fmt.Errorf("error writing persistence file: %w", err)
	}

	return nil
}

// UpdateRegister updates a Modbus holding register with a 32-bit signed integer value
func (sc *ServerContext) UpdateRegister(unitID int, registerOffset int, value int32) {
	slave, ok := sc.Slaves[unitID]
	if !ok {
		sc.logger.Warn("Unit ID not found",
			zap.Int("unitID", unitID))
		return
	}

	// Convert int32 to two uint16 values (big endian)
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(value))
	r1 := binary.BigEndian.Uint16(buf[0:2])
	r2 := binary.BigEndian.Uint16(buf[2:4])

	slave.Mu.Lock()
	defer slave.Mu.Unlock()

	if registerOffset < 0 || registerOffset+1 >= len(slave.HR) {
		sc.logger.Warn("Register offset out of range",
			zap.Int("unitID", unitID),
			zap.Int("offset", registerOffset))
		return
	}

	slave.HR[registerOffset] = r1
	slave.HR[registerOffset+1] = r2

	// Save the value
	if err := sc.SaveValue(unitID, registerOffset, value); err != nil {
		sc.logger.Error("Error saving value",
			zap.Error(err))
	}

	sc.logger.Info("Updated Modbus register",
		zap.Int("unitID", unitID),
		zap.Int("offset", registerOffset),
		zap.Int32("value", value))
}
