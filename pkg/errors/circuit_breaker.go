package errors

import (
	"fmt"
	"sync"
	"time"
)

// CircuitBreakerState represents the state of a circuit breaker
type CircuitBreakerState int

const (
	// CircuitBreakerClosed - normal operation, requests are allowed
	CircuitBreakerClosed CircuitBreakerState = iota
	// CircuitBreakerOpen - circuit is open, requests are rejected
	CircuitBreakerOpen
	// CircuitBreakerHalfOpen - testing if the service has recovered
	CircuitBreakerHalfOpen
)

// String returns the string representation of the circuit breaker state
func (s CircuitBreakerState) String() string {
	switch s {
	case CircuitBreakerClosed:
		return "CLOSED"
	case CircuitBreakerOpen:
		return "OPEN"
	case CircuitBreakerHalfOpen:
		return "HALF_OPEN"
	default:
		return "UNKNOWN"
	}
}

// CircuitBreakerConfig holds configuration for a circuit breaker
type CircuitBreakerConfig struct {
	// MaxFailures is the maximum number of failures before opening the circuit
	MaxFailures int
	// ResetTimeout is the time to wait before transitioning from Open to Half-Open
	ResetTimeout time.Duration
	// SuccessThreshold is the number of consecutive successes needed to close the circuit from Half-Open
	SuccessThreshold int
	// Name is the identifier for this circuit breaker
	Name string
}

// DefaultCircuitBreakerConfig returns a default configuration
func DefaultCircuitBreakerConfig(name string) CircuitBreakerConfig {
	return CircuitBreakerConfig{
		MaxFailures:      5,
		ResetTimeout:     30 * time.Second,
		SuccessThreshold: 3,
		Name:             name,
	}
}

// CircuitBreaker implements the circuit breaker pattern for fault tolerance
type CircuitBreaker struct {
	config          CircuitBreakerConfig
	state           CircuitBreakerState
	failureCount    int
	successCount    int
	lastFailureTime time.Time
	mutex           sync.RWMutex
	onStateChange   func(from, to CircuitBreakerState)
}

// NewCircuitBreaker creates a new circuit breaker with the given configuration
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config: config,
		state:  CircuitBreakerClosed,
	}
}

// SetStateChangeCallback sets a callback function that is called when the circuit breaker state changes
func (cb *CircuitBreaker) SetStateChangeCallback(callback func(from, to CircuitBreakerState)) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.onStateChange = callback
}

// Execute executes the given function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.allowRequest() {
		return NewSystemError("CIRCUIT_BREAKER_OPEN",
			fmt.Sprintf("Circuit breaker '%s' is open", cb.config.Name), nil).
			WithContext("circuit_breaker", cb.config.Name).
			WithContext("state", cb.GetState().String())
	}

	err := fn()
	cb.recordResult(err)
	return err
}

// allowRequest determines if a request should be allowed based on the current state
func (cb *CircuitBreaker) allowRequest() bool {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	switch cb.state {
	case CircuitBreakerClosed:
		return true
	case CircuitBreakerOpen:
		// Check if we should transition to half-open
		if time.Since(cb.lastFailureTime) >= cb.config.ResetTimeout {
			cb.setState(CircuitBreakerHalfOpen)
			return true
		}
		return false
	case CircuitBreakerHalfOpen:
		return true
	default:
		return false
	}
}

// recordResult records the result of an operation and updates the circuit breaker state
func (cb *CircuitBreaker) recordResult(err error) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}
}

// recordFailure records a failure and potentially opens the circuit
func (cb *CircuitBreaker) recordFailure() {
	cb.failureCount++
	cb.successCount = 0
	cb.lastFailureTime = time.Now()

	switch cb.state {
	case CircuitBreakerClosed:
		if cb.failureCount >= cb.config.MaxFailures {
			cb.setState(CircuitBreakerOpen)
		}
	case CircuitBreakerHalfOpen:
		cb.setState(CircuitBreakerOpen)
	}
}

// recordSuccess records a success and potentially closes the circuit
func (cb *CircuitBreaker) recordSuccess() {
	cb.successCount++

	switch cb.state {
	case CircuitBreakerHalfOpen:
		if cb.successCount >= cb.config.SuccessThreshold {
			cb.setState(CircuitBreakerClosed)
			cb.reset()
		}
	case CircuitBreakerClosed:
		// Reset failure count on success in closed state
		cb.failureCount = 0
	}
}

// setState changes the circuit breaker state and calls the callback if set
func (cb *CircuitBreaker) setState(newState CircuitBreakerState) {
	oldState := cb.state
	cb.state = newState

	if cb.onStateChange != nil && oldState != newState {
		// Call callback without holding the lock to avoid deadlocks
		go cb.onStateChange(oldState, newState)
	}
}

// reset resets the circuit breaker counters
func (cb *CircuitBreaker) reset() {
	cb.failureCount = 0
	cb.successCount = 0
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()
	return cb.state
}

// GetStats returns statistics about the circuit breaker
func (cb *CircuitBreaker) GetStats() CircuitBreakerStats {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	return CircuitBreakerStats{
		Name:            cb.config.Name,
		State:           cb.state,
		FailureCount:    cb.failureCount,
		SuccessCount:    cb.successCount,
		LastFailureTime: cb.lastFailureTime,
		Config:          cb.config,
	}
}

// CircuitBreakerStats holds statistics about a circuit breaker
type CircuitBreakerStats struct {
	Name            string               `json:"name"`
	State           CircuitBreakerState  `json:"state"`
	FailureCount    int                  `json:"failureCount"`
	SuccessCount    int                  `json:"successCount"`
	LastFailureTime time.Time            `json:"lastFailureTime"`
	Config          CircuitBreakerConfig `json:"config"`
}

// IsHealthy returns true if the circuit breaker is in a healthy state
func (stats CircuitBreakerStats) IsHealthy() bool {
	return stats.State == CircuitBreakerClosed
}

// CircuitBreakerManager manages multiple circuit breakers
type CircuitBreakerManager struct {
	breakers map[string]*CircuitBreaker
	mutex    sync.RWMutex
}

// NewCircuitBreakerManager creates a new circuit breaker manager
func NewCircuitBreakerManager() *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// GetOrCreate gets an existing circuit breaker or creates a new one
func (cbm *CircuitBreakerManager) GetOrCreate(name string, config CircuitBreakerConfig) *CircuitBreaker {
	cbm.mutex.Lock()
	defer cbm.mutex.Unlock()

	if breaker, exists := cbm.breakers[name]; exists {
		return breaker
	}

	config.Name = name
	breaker := NewCircuitBreaker(config)
	cbm.breakers[name] = breaker
	return breaker
}

// Get retrieves a circuit breaker by name
func (cbm *CircuitBreakerManager) Get(name string) (*CircuitBreaker, bool) {
	cbm.mutex.RLock()
	defer cbm.mutex.RUnlock()

	breaker, exists := cbm.breakers[name]
	return breaker, exists
}

// GetAllStats returns statistics for all circuit breakers
func (cbm *CircuitBreakerManager) GetAllStats() map[string]CircuitBreakerStats {
	cbm.mutex.RLock()
	defer cbm.mutex.RUnlock()

	stats := make(map[string]CircuitBreakerStats)
	for name, breaker := range cbm.breakers {
		stats[name] = breaker.GetStats()
	}
	return stats
}

// GetHealthyBreakers returns the names of all healthy circuit breakers
func (cbm *CircuitBreakerManager) GetHealthyBreakers() []string {
	cbm.mutex.RLock()
	defer cbm.mutex.RUnlock()

	var healthy []string
	for name, breaker := range cbm.breakers {
		if breaker.GetStats().IsHealthy() {
			healthy = append(healthy, name)
		}
	}
	return healthy
}

// GetUnhealthyBreakers returns the names of all unhealthy circuit breakers
func (cbm *CircuitBreakerManager) GetUnhealthyBreakers() []string {
	cbm.mutex.RLock()
	defer cbm.mutex.RUnlock()

	var unhealthy []string
	for name, breaker := range cbm.breakers {
		if !breaker.GetStats().IsHealthy() {
			unhealthy = append(unhealthy, name)
		}
	}
	return unhealthy
}
