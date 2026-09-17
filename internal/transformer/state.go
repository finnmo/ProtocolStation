package transformer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// StateStorage provides thread-safe file-based state storage for transformers
type StateStorage struct {
	filePath string
	data     map[string]interface{}
	mu       sync.RWMutex
	logger   *zap.Logger
	dirty    bool
}

// NewStateStorage creates a new state storage instance
func NewStateStorage(filePath string, logger *zap.Logger) (*StateStorage, error) {
	ss := &StateStorage{
		filePath: filePath,
		data:     make(map[string]interface{}),
		logger:   logger,
	}

	// Load existing state if file exists
	if err := ss.Load(); err != nil {
		// If file doesn't exist, start with empty state
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load state: %w", err)
		}
		ss.logger.Debug("state file does not exist, starting with empty state",
			zap.String("file", filePath))
	}

	return ss, nil
}

// Load reads state from file
func (s *StateStorage) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		s.data = make(map[string]interface{})
		return nil
	}

	if err := json.Unmarshal(data, &s.data); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	s.logger.Debug("loaded state from file",
		zap.String("file", s.filePath),
		zap.Int("keys", len(s.data)))
	return nil
}

// save writes state to disk. Caller must hold s.mu (write lock).
func (s *StateStorage) save() error {
	if !s.dirty {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(s.filePath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(s.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	s.dirty = false
	s.logger.Debug("saved state to file",
		zap.String("file", s.filePath),
		zap.Int("keys", len(s.data)))
	return nil
}

// Save writes state to file
func (s *StateStorage) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.save()
}

// Get retrieves a value from state
func (s *StateStorage) Get(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.data[key]
	return value, ok
}

// Set stores a value in state and marks as dirty
func (s *StateStorage) Set(key string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	s.dirty = true
	return nil
}

// SetAndSave stores a value and immediately saves to disk in a single atomic operation.
// Holds the write lock across both the map update and the file write so no
// concurrent Get/Set can observe a partial update.
func (s *StateStorage) SetAndSave(key string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	s.dirty = true
	return s.save()
}

// GetAll returns a copy of all state data
func (s *StateStorage) GetAll() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make(map[string]interface{})
	for k, v := range s.data {
		result[k] = v
	}
	return result
}

// AutoSave starts a goroutine that periodically saves state if dirty
func (s *StateStorage) AutoSave(interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			// Final save before stopping
			if err := s.Save(); err != nil {
				s.logger.Error("failed to save state on shutdown", zap.Error(err))
			}
			return
		case <-ticker.C:
			if err := s.Save(); err != nil {
				s.logger.Error("failed to auto-save state", zap.Error(err))
			}
		}
	}
}
