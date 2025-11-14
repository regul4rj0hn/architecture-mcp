package errors

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// executeFailingOperations executes multiple failing operations on a circuit breaker
func executeFailingOperations(t *testing.T, cb *CircuitBreaker, count int) {
	t.Helper()
	for range count {
		err := cb.Execute(func() error {
			return fmt.Errorf("test error")
		})
		if err == nil {
			t.Errorf("Expected error from failing operation")
		}
	}
}

// openCircuit opens a circuit breaker by executing failing operations
func openCircuit(t *testing.T, cb *CircuitBreaker, failureCount int) {
	t.Helper()
	executeFailingOperations(t, cb, failureCount)
	if cb.GetState() != CircuitBreakerOpen {
		t.Fatalf("Failed to open circuit, state: %s", cb.GetState())
	}
}

func TestCircuitBreakerInitialState(t *testing.T) {
	config := DefaultCircuitBreakerConfig("test")
	cb := NewCircuitBreaker(config)

	if cb.GetState() != CircuitBreakerClosed {
		t.Errorf("Expected initial state to be CLOSED, got %s", cb.GetState())
	}
}

func TestCircuitBreakerOpensAfterFailures(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:      3,
		ResetTimeout:     1 * time.Second,
		SuccessThreshold: 2,
		Name:             "test",
	}
	cb := NewCircuitBreaker(config)

	executeFailingOperations(t, cb, 3)

	if cb.GetState() != CircuitBreakerOpen {
		t.Errorf("Expected state to be OPEN after max failures, got %s", cb.GetState())
	}
}

func TestCircuitBreakerRejectsWhenOpen(t *testing.T) {
	config := CircuitBreakerConfig{
		MaxFailures:      2,
		ResetTimeout:     1 * time.Second,
		SuccessThreshold: 1,
		Name:             "test",
	}
	cb := NewCircuitBreaker(config)

	executeFailingOperations(t, cb, 2)

	// Try to execute when circuit is open
	err := cb.Execute(func() error {
		return nil
	})

	if err == nil {
		t.Errorf("Expected circuit breaker to reject request when open")
	}

	structuredErr, ok := err.(*StructuredError)
	if !ok {
		t.Errorf("Expected structured error from circuit breaker")
	}
	if structuredErr.Code != "CIRCUIT_BREAKER_OPEN" {
		t.Errorf("Expected circuit breaker open error code, got %s", structuredErr.Code)
	}
}

func TestCircuitBreakerStateTransitions(t *testing.T) {
	tests := []struct {
		name             string
		successThreshold int
		successCount     int
		failAfterTimeout bool
		expectedState    CircuitBreakerState
		shouldExecute    bool
	}{
		{
			name:             "transitions to half-open after timeout",
			successThreshold: 1,
			successCount:     1,
			failAfterTimeout: false,
			expectedState:    CircuitBreakerClosed,
			shouldExecute:    true,
		},
		{
			name:             "closes after success threshold met",
			successThreshold: 2,
			successCount:     2,
			failAfterTimeout: false,
			expectedState:    CircuitBreakerClosed,
			shouldExecute:    true,
		},
		{
			name:             "reopens on failure in half-open",
			successThreshold: 2,
			successCount:     0,
			failAfterTimeout: true,
			expectedState:    CircuitBreakerOpen,
			shouldExecute:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := CircuitBreakerConfig{
				MaxFailures:      2,
				ResetTimeout:     100 * time.Millisecond,
				SuccessThreshold: tt.successThreshold,
				Name:             "test",
			}
			cb := NewCircuitBreaker(config)

			openCircuit(t, cb, 2)
			time.Sleep(150 * time.Millisecond)

			if tt.failAfterTimeout {
				cb.Execute(func() error {
					return fmt.Errorf("failure in half-open")
				})
			} else {
				executed := false
				for range tt.successCount {
					err := cb.Execute(func() error {
						executed = true
						return nil
					})
					if err != nil {
						t.Errorf("Expected successful execution, got error: %v", err)
					}
				}
				if tt.shouldExecute && !executed {
					t.Errorf("Expected function to be executed")
				}
			}

			if cb.GetState() != tt.expectedState {
				t.Errorf("Expected state %s, got %s", tt.expectedState, cb.GetState())
			}
		})
	}
}

func TestCircuitBreakerStats(t *testing.T) {
	t.Run("GetStats returns correct information", func(t *testing.T) {
		config := CircuitBreakerConfig{
			MaxFailures:      3,
			ResetTimeout:     1 * time.Second,
			SuccessThreshold: 2,
			Name:             "test-stats",
		}
		cb := NewCircuitBreaker(config)

		// Execute some operations - failure first, then success
		cb.Execute(func() error { return fmt.Errorf("error") })

		stats := cb.GetStats()

		if stats.Name != "test-stats" {
			t.Errorf("Expected name 'test-stats', got %s", stats.Name)
		}
		if stats.State != CircuitBreakerClosed {
			t.Errorf("Expected state CLOSED, got %s", stats.State)
		}
		if stats.FailureCount != 1 {
			t.Errorf("Expected failure count 1, got %d", stats.FailureCount)
		}

		// Execute success - this will reset failure count in closed state
		cb.Execute(func() error { return nil })

		stats = cb.GetStats()
		if stats.SuccessCount != 1 {
			t.Errorf("Expected success count 1, got %d", stats.SuccessCount)
		}
		// Failure count should be reset to 0 after success in closed state
		if stats.FailureCount != 0 {
			t.Errorf("Expected failure count 0 after success in closed state, got %d", stats.FailureCount)
		}
	})

	t.Run("IsHealthy returns correct status", func(t *testing.T) {
		config := DefaultCircuitBreakerConfig("test")
		cb := NewCircuitBreaker(config)

		stats := cb.GetStats()
		if !stats.IsHealthy() {
			t.Errorf("Expected healthy circuit breaker to report as healthy")
		}

		// Open the circuit
		for i := 0; i < config.MaxFailures; i++ {
			cb.Execute(func() error {
				return fmt.Errorf("test error")
			})
		}

		stats = cb.GetStats()
		if stats.IsHealthy() {
			t.Errorf("Expected open circuit breaker to report as unhealthy")
		}
	})
}

func TestCircuitBreakerStateChangeCallback(t *testing.T) {
	t.Run("State change callback is called", func(t *testing.T) {
		config := CircuitBreakerConfig{
			MaxFailures:      2,
			ResetTimeout:     100 * time.Millisecond,
			SuccessThreshold: 1,
			Name:             "test-callback",
		}
		cb := NewCircuitBreaker(config)

		var mu sync.Mutex
		var callbackCalled bool
		var fromState, toState CircuitBreakerState

		cb.SetStateChangeCallback(func(from, to CircuitBreakerState) {
			mu.Lock()
			callbackCalled = true
			fromState = from
			toState = to
			mu.Unlock()
		})

		// Open the circuit
		for range 2 {
			cb.Execute(func() error {
				return fmt.Errorf("test error")
			})
		}

		// Give callback time to execute (it runs in goroutine)
		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		called := callbackCalled
		from := fromState
		to := toState
		mu.Unlock()

		if !called {
			t.Errorf("Expected state change callback to be called")
		}
		if from != CircuitBreakerClosed {
			t.Errorf("Expected from state to be CLOSED, got %s", from)
		}
		if to != CircuitBreakerOpen {
			t.Errorf("Expected to state to be OPEN, got %s", to)
		}
	})
}

func TestCircuitBreakerManager(t *testing.T) {
	t.Run("GetOrCreate returns same instance for same name", func(t *testing.T) {
		manager := NewCircuitBreakerManager()
		config := DefaultCircuitBreakerConfig("test")

		cb1 := manager.GetOrCreate("test", config)
		cb2 := manager.GetOrCreate("test", config)

		if cb1 != cb2 {
			t.Errorf("Expected GetOrCreate to return same instance for same name")
		}
	})

	t.Run("Get returns correct circuit breaker", func(t *testing.T) {
		manager := NewCircuitBreakerManager()
		config := DefaultCircuitBreakerConfig("test")

		original := manager.GetOrCreate("test", config)
		retrieved, exists := manager.Get("test")

		if !exists {
			t.Errorf("Expected circuit breaker to exist")
		}
		if retrieved != original {
			t.Errorf("Expected Get to return same instance")
		}
	})

	t.Run("GetAllStats returns stats for all breakers", func(t *testing.T) {
		manager := NewCircuitBreakerManager()

		manager.GetOrCreate("breaker1", DefaultCircuitBreakerConfig("breaker1"))
		manager.GetOrCreate("breaker2", DefaultCircuitBreakerConfig("breaker2"))

		stats := manager.GetAllStats()

		if len(stats) != 2 {
			t.Errorf("Expected 2 circuit breakers, got %d", len(stats))
		}
		if _, exists := stats["breaker1"]; !exists {
			t.Errorf("Expected breaker1 in stats")
		}
		if _, exists := stats["breaker2"]; !exists {
			t.Errorf("Expected breaker2 in stats")
		}
	})

	t.Run("GetHealthyBreakers returns only healthy breakers", func(t *testing.T) {
		manager := NewCircuitBreakerManager()

		config := CircuitBreakerConfig{
			MaxFailures:      2,
			ResetTimeout:     1 * time.Second,
			SuccessThreshold: 1,
			Name:             "",
		}

		_ = manager.GetOrCreate("healthy", config)
		unhealthy := manager.GetOrCreate("unhealthy", config)

		// Make one unhealthy
		for range 2 {
			unhealthy.Execute(func() error {
				return fmt.Errorf("error")
			})
		}

		healthyNames := manager.GetHealthyBreakers()

		if len(healthyNames) != 1 {
			t.Errorf("Expected 1 healthy breaker, got %d", len(healthyNames))
		}
		if healthyNames[0] != "healthy" {
			t.Errorf("Expected healthy breaker to be 'healthy', got %s", healthyNames[0])
		}
	})
}
