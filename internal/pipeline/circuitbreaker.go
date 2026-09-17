package pipeline

import (
	"sync"
	"time"
)

// CircuitBreaker implements the circuit breaker pattern for preventing cascading failures
type CircuitBreaker struct {
	name            string
	maxFailures     int
	timeout         time.Duration
	resetTimeout    time.Duration
	failureCount    int
	lastFailureTime time.Time
	state           CircuitState
	mu              sync.RWMutex
}

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	StateClosed CircuitState = iota
	StateOpen
	StateHalfOpen
)

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(name string, maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:         name,
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		state:        StateClosed,
	}
}

// Call attempts to execute a function with circuit breaker protection
func (cb *CircuitBreaker) Call(fn func() error) error {
	cb.mu.RLock()
	state := cb.state
	lastFailureTime := cb.lastFailureTime
	cb.mu.RUnlock()

	// Check if circuit should be closed
	if state == StateOpen {
		if time.Since(lastFailureTime) >= cb.resetTimeout {
			cb.mu.Lock()
			if cb.state == StateOpen {
				cb.state = StateHalfOpen
			}
			cb.mu.Unlock()
			state = StateHalfOpen
		} else {
			return &ErrCircuitOpen{Name: cb.name}
		}
	}

	// Attempt to execute the function
	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failureCount++
		cb.lastFailureTime = time.Now()

		if cb.failureCount >= cb.maxFailures {
			cb.state = StateOpen
		}
		return err
	}

	// Success - reset failure count
	if cb.state == StateHalfOpen || cb.state == StateOpen {
		cb.state = StateClosed
	}
	cb.failureCount = 0
	return nil
}

// State returns the current state of the circuit breaker
func (cb *CircuitBreaker) State() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset manually resets the circuit breaker
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failureCount = 0
}

// ErrCircuitOpen is returned when the circuit is open
type ErrCircuitOpen struct {
	Name string
}

func (e ErrCircuitOpen) Error() string {
	return "circuit breaker is open: " + e.Name
}
