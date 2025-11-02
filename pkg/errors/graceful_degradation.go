package errors

import (
	"fmt"
	"sync"
	"time"
)

// DegradationLevel represents the level of service degradation
type DegradationLevel int

const (
	// DegradationNone - full service functionality
	DegradationNone DegradationLevel = iota
	// DegradationMinor - minor features disabled
	DegradationMinor
	// DegradationMajor - major features disabled, core functionality only
	DegradationMajor
	// DegradationCritical - minimal functionality, emergency mode
	DegradationCritical
)

// String returns the string representation of the degradation level
func (d DegradationLevel) String() string {
	switch d {
	case DegradationNone:
		return "NONE"
	case DegradationMinor:
		return "MINOR"
	case DegradationMajor:
		return "MAJOR"
	case DegradationCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// ServiceComponent represents a component that can be degraded
type ServiceComponent string

const (
	ComponentFileSystemMonitoring ServiceComponent = "filesystem_monitoring"
	ComponentCacheRefresh         ServiceComponent = "cache_refresh"
	ComponentDocumentParsing      ServiceComponent = "document_parsing"
	ComponentResourceDiscovery    ServiceComponent = "resource_discovery"
)

// DegradationRule defines when and how to degrade a service component
type DegradationRule struct {
	Component         ServiceComponent
	ErrorThreshold    int
	TimeWindow        time.Duration
	DegradationLevel  DegradationLevel
	FallbackBehavior  func() error
	RecoveryCondition func() bool
}

// ComponentStatus tracks the status of a service component
type ComponentStatus struct {
	Component        ServiceComponent `json:"component"`
	DegradationLevel DegradationLevel `json:"degradationLevel"`
	ErrorCount       int              `json:"errorCount"`
	LastError        time.Time        `json:"lastError"`
	LastRecovery     time.Time        `json:"lastRecovery"`
	IsHealthy        bool             `json:"isHealthy"`
	Message          string           `json:"message"`
}

// GracefulDegradationManager manages service degradation based on error patterns
type GracefulDegradationManager struct {
	components    map[ServiceComponent]*ComponentStatus
	rules         map[ServiceComponent]*DegradationRule
	errorHistory  map[ServiceComponent][]time.Time
	mutex         sync.RWMutex
	onStateChange func(component ServiceComponent, oldLevel, newLevel DegradationLevel)
}

// NewGracefulDegradationManager creates a new graceful degradation manager
func NewGracefulDegradationManager() *GracefulDegradationManager {
	return &GracefulDegradationManager{
		components:   make(map[ServiceComponent]*ComponentStatus),
		rules:        make(map[ServiceComponent]*DegradationRule),
		errorHistory: make(map[ServiceComponent][]time.Time),
	}
}

// RegisterComponent registers a component with degradation rules
func (gdm *GracefulDegradationManager) RegisterComponent(rule DegradationRule) {
	gdm.mutex.Lock()
	defer gdm.mutex.Unlock()

	gdm.rules[rule.Component] = &rule
	gdm.components[rule.Component] = &ComponentStatus{
		Component:        rule.Component,
		DegradationLevel: DegradationNone,
		IsHealthy:        true,
		Message:          "Component operating normally",
	}
	gdm.errorHistory[rule.Component] = make([]time.Time, 0)
}

// SetStateChangeCallback sets a callback for when component degradation levels change
func (gdm *GracefulDegradationManager) SetStateChangeCallback(callback func(ServiceComponent, DegradationLevel, DegradationLevel)) {
	gdm.mutex.Lock()
	defer gdm.mutex.Unlock()
	gdm.onStateChange = callback
}

// RecordError records an error for a component and potentially triggers degradation
func (gdm *GracefulDegradationManager) RecordError(component ServiceComponent, err error) {
	gdm.mutex.Lock()
	defer gdm.mutex.Unlock()

	status, exists := gdm.components[component]
	if !exists {
		return // Component not registered
	}

	rule := gdm.rules[component]
	now := time.Now()

	// Add error to history
	gdm.errorHistory[component] = append(gdm.errorHistory[component], now)

	// Clean old errors outside the time window
	gdm.cleanErrorHistory(component, rule.TimeWindow)

	// Update component status
	status.ErrorCount = len(gdm.errorHistory[component])
	status.LastError = now

	// Check if degradation is needed
	oldLevel := status.DegradationLevel
	if status.ErrorCount >= rule.ErrorThreshold {
		gdm.degradeComponent(component, rule.DegradationLevel)
	}

	// Trigger callback if level changed
	if gdm.onStateChange != nil && oldLevel != status.DegradationLevel {
		go gdm.onStateChange(component, oldLevel, status.DegradationLevel)
	}
}

// RecordSuccess records a successful operation for a component
func (gdm *GracefulDegradationManager) RecordSuccess(component ServiceComponent) {
	gdm.mutex.Lock()
	defer gdm.mutex.Unlock()

	status, exists := gdm.components[component]
	if !exists {
		return
	}

	rule := gdm.rules[component]

	// Check recovery condition if component is degraded
	if status.DegradationLevel != DegradationNone && rule.RecoveryCondition != nil {
		if rule.RecoveryCondition() {
			gdm.recoverComponent(component)
		}
	}
}

// degradeComponent degrades a component to the specified level
func (gdm *GracefulDegradationManager) degradeComponent(component ServiceComponent, level DegradationLevel) {
	status := gdm.components[component]

	status.DegradationLevel = level
	status.IsHealthy = level == DegradationNone
	status.Message = fmt.Sprintf("Component degraded to %s due to repeated errors", level.String())

	// Execute fallback behavior if available
	rule := gdm.rules[component]
	if rule.FallbackBehavior != nil {
		go func() {
			if err := rule.FallbackBehavior(); err != nil {
				// Log fallback error but don't cascade
				fmt.Printf("Fallback behavior failed for component %s: %v\n", component, err)
			}
		}()
	}
}

// recoverComponent recovers a component to normal operation
func (gdm *GracefulDegradationManager) recoverComponent(component ServiceComponent) {
	status := gdm.components[component]
	status.DegradationLevel = DegradationNone
	status.IsHealthy = true
	status.LastRecovery = time.Now()
	status.Message = "Component recovered to normal operation"

	// Clear error history on recovery
	gdm.errorHistory[component] = make([]time.Time, 0)
	status.ErrorCount = 0
}

// cleanErrorHistory removes errors outside the time window
func (gdm *GracefulDegradationManager) cleanErrorHistory(component ServiceComponent, window time.Duration) {
	cutoff := time.Now().Add(-window)
	history := gdm.errorHistory[component]

	var cleaned []time.Time
	for _, errorTime := range history {
		if errorTime.After(cutoff) {
			cleaned = append(cleaned, errorTime)
		}
	}

	gdm.errorHistory[component] = cleaned
}

// GetComponentStatus returns the current status of a component
func (gdm *GracefulDegradationManager) GetComponentStatus(component ServiceComponent) (*ComponentStatus, bool) {
	gdm.mutex.RLock()
	defer gdm.mutex.RUnlock()

	status, exists := gdm.components[component]
	if !exists {
		return nil, false
	}

	// Return a copy to avoid concurrent access issues
	statusCopy := *status
	return &statusCopy, true
}

// GetAllComponentStatuses returns the status of all registered components
func (gdm *GracefulDegradationManager) GetAllComponentStatuses() map[ServiceComponent]*ComponentStatus {
	gdm.mutex.RLock()
	defer gdm.mutex.RUnlock()

	result := make(map[ServiceComponent]*ComponentStatus)
	for component, status := range gdm.components {
		statusCopy := *status
		result[component] = &statusCopy
	}
	return result
}

// IsComponentHealthy returns true if the component is operating normally
func (gdm *GracefulDegradationManager) IsComponentHealthy(component ServiceComponent) bool {
	gdm.mutex.RLock()
	defer gdm.mutex.RUnlock()

	status, exists := gdm.components[component]
	return exists && status.IsHealthy
}

// GetOverallHealth returns the overall system health based on component statuses
func (gdm *GracefulDegradationManager) GetOverallHealth() DegradationLevel {
	gdm.mutex.RLock()
	defer gdm.mutex.RUnlock()

	maxDegradation := DegradationNone
	for _, status := range gdm.components {
		if status.DegradationLevel > maxDegradation {
			maxDegradation = status.DegradationLevel
		}
	}
	return maxDegradation
}

// ExecuteWithDegradation executes a function with degradation awareness
func (gdm *GracefulDegradationManager) ExecuteWithDegradation(
	component ServiceComponent,
	normalFn func() error,
	degradedFn func(level DegradationLevel) error,
) error {
	status, exists := gdm.GetComponentStatus(component)
	if !exists {
		return NewSystemError("COMPONENT_NOT_REGISTERED",
			fmt.Sprintf("Component %s not registered for degradation management", component), nil)
	}

	if status.DegradationLevel == DegradationNone {
		err := normalFn()
		if err != nil {
			gdm.RecordError(component, err)
		} else {
			gdm.RecordSuccess(component)
		}
		return err
	}

	// Execute degraded function
	if degradedFn != nil {
		return degradedFn(status.DegradationLevel)
	}

	// No degraded function provided, return error
	return NewSystemError("SERVICE_DEGRADED",
		fmt.Sprintf("Component %s is degraded (%s) and no fallback provided",
			component, status.DegradationLevel.String()), nil).
		WithContext("component", string(component)).
		WithContext("degradation_level", status.DegradationLevel.String())
}

// ForceRecovery forces a component to recover to normal operation
func (gdm *GracefulDegradationManager) ForceRecovery(component ServiceComponent) error {
	gdm.mutex.Lock()
	defer gdm.mutex.Unlock()

	status, exists := gdm.components[component]
	if !exists {
		return NewSystemError("COMPONENT_NOT_FOUND",
			fmt.Sprintf("Component %s not found", component), nil)
	}

	oldLevel := status.DegradationLevel
	gdm.recoverComponent(component)

	if gdm.onStateChange != nil && oldLevel != DegradationNone {
		go gdm.onStateChange(component, oldLevel, DegradationNone)
	}

	return nil
}

// CreateDefaultRules creates default degradation rules for common components
func CreateDefaultRules() []DegradationRule {
	return []DegradationRule{
		{
			Component:        ComponentFileSystemMonitoring,
			ErrorThreshold:   3,
			TimeWindow:       5 * time.Minute,
			DegradationLevel: DegradationMinor,
			FallbackBehavior: func() error {
				// Fallback: disable real-time monitoring, rely on periodic scans
				return nil
			},
			RecoveryCondition: func() bool {
				// Recover after 2 minutes of no errors
				return true
			},
		},
		{
			Component:        ComponentCacheRefresh,
			ErrorThreshold:   5,
			TimeWindow:       3 * time.Minute,
			DegradationLevel: DegradationMajor,
			FallbackBehavior: func() error {
				// Fallback: disable automatic cache refresh
				return nil
			},
			RecoveryCondition: func() bool {
				return true
			},
		},
		{
			Component:        ComponentDocumentParsing,
			ErrorThreshold:   10,
			TimeWindow:       10 * time.Minute,
			DegradationLevel: DegradationMinor,
			FallbackBehavior: func() error {
				// Fallback: serve cached content only
				return nil
			},
			RecoveryCondition: func() bool {
				return true
			},
		},
		{
			Component:        ComponentResourceDiscovery,
			ErrorThreshold:   2,
			TimeWindow:       1 * time.Minute,
			DegradationLevel: DegradationCritical,
			FallbackBehavior: func() error {
				// Fallback: return empty resource list
				return nil
			},
			RecoveryCondition: func() bool {
				return true
			},
		},
	}
}
