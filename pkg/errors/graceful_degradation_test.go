package errors

import (
	"testing"
	"time"
)

func TestGracefulDegradationManager(t *testing.T) {
	t.Run("RegisterComponent creates component with correct initial state", func(t *testing.T) {
		manager := NewGracefulDegradationManager()

		rule := DegradationRule{
			Component:        ComponentFileSystemMonitoring,
			ErrorThreshold:   3,
			TimeWindow:       5 * time.Minute,
			DegradationLevel: DegradationMinor,
		}

		manager.RegisterComponent(rule)

		status, exists := manager.GetComponentStatus(ComponentFileSystemMonitoring)
		if !exists {
			t.Errorf("Expected component to be registered")
		}
		if status.DegradationLevel != DegradationNone {
			t.Errorf("Expected initial degradation level to be NONE, got %s", status.DegradationLevel)
		}
		if !status.IsHealthy {
			t.Errorf("Expected component to be initially healthy")
		}
	})

	t.Run("RecordError triggers degradation after threshold", func(t *testing.T) {
		manager := NewGracefulDegradationManager()

		rule := DegradationRule{
			Component:        ComponentCacheRefresh,
			ErrorThreshold:   2,
			TimeWindow:       1 * time.Minute,
			DegradationLevel: DegradationMajor,
		}

		manager.RegisterComponent(rule)

		// Record errors up to threshold
		for i := 0; i < 2; i++ {
			manager.RecordError(ComponentCacheRefresh, NewCacheError("TEST", "Test error", nil))
		}

		status, _ := manager.GetComponentStatus(ComponentCacheRefresh)
		if status.DegradationLevel != DegradationMajor {
			t.Errorf("Expected degradation level MAJOR after threshold, got %s", status.DegradationLevel)
		}
		if status.IsHealthy {
			t.Errorf("Expected component to be unhealthy after degradation")
		}
	})

	t.Run("Old errors outside time window are cleaned up", func(t *testing.T) {
		manager := NewGracefulDegradationManager()

		rule := DegradationRule{
			Component:        ComponentDocumentParsing,
			ErrorThreshold:   3,
			TimeWindow:       100 * time.Millisecond, // Very short window
			DegradationLevel: DegradationMinor,
		}

		manager.RegisterComponent(rule)

		// Record errors
		for i := 0; i < 2; i++ {
			manager.RecordError(ComponentDocumentParsing, NewParsingError("TEST", "Test error", nil))
		}

		// Wait for time window to pass
		time.Sleep(150 * time.Millisecond)

		// Record one more error (should not trigger degradation since old errors are cleaned)
		manager.RecordError(ComponentDocumentParsing, NewParsingError("TEST", "Test error", nil))

		status, _ := manager.GetComponentStatus(ComponentDocumentParsing)
		if status.DegradationLevel != DegradationNone {
			t.Errorf("Expected degradation level NONE after time window cleanup, got %s", status.DegradationLevel)
		}
	})

	t.Run("RecordSuccess triggers recovery when condition is met", func(t *testing.T) {
		manager := NewGracefulDegradationManager()

		recoveryTriggered := false
		rule := DegradationRule{
			Component:        ComponentResourceDiscovery,
			ErrorThreshold:   1,
			TimeWindow:       1 * time.Minute,
			DegradationLevel: DegradationCritical,
			RecoveryCondition: func() bool {
				return recoveryTriggered
			},
		}

		manager.RegisterComponent(rule)

		// Trigger degradation
		manager.RecordError(ComponentResourceDiscovery, NewMCPError("TEST", "Test error", nil))

		status, _ := manager.GetComponentStatus(ComponentResourceDiscovery)
		if status.DegradationLevel != DegradationCritical {
			t.Errorf("Expected degradation level CRITICAL, got %s", status.DegradationLevel)
		}

		// Record success but recovery condition not met
		manager.RecordSuccess(ComponentResourceDiscovery)

		status, _ = manager.GetComponentStatus(ComponentResourceDiscovery)
		if status.DegradationLevel != DegradationCritical {
			t.Errorf("Expected degradation level to remain CRITICAL when recovery condition not met")
		}

		// Set recovery condition and record success
		recoveryTriggered = true
		manager.RecordSuccess(ComponentResourceDiscovery)

		status, _ = manager.GetComponentStatus(ComponentResourceDiscovery)
		if status.DegradationLevel != DegradationNone {
			t.Errorf("Expected degradation level NONE after recovery, got %s", status.DegradationLevel)
		}
	})

	t.Run("GetOverallHealth returns maximum degradation level", func(t *testing.T) {
		manager := NewGracefulDegradationManager()

		// Register multiple components
		rules := []DegradationRule{
			{
				Component:        ComponentFileSystemMonitoring,
				ErrorThreshold:   1,
				TimeWindow:       1 * time.Minute,
				DegradationLevel: DegradationMinor,
			},
			{
				Component:        ComponentCacheRefresh,
				ErrorThreshold:   1,
				TimeWindow:       1 * time.Minute,
				DegradationLevel: DegradationMajor,
			},
		}

		for _, rule := range rules {
			manager.RegisterComponent(rule)
		}

		// Initially all healthy
		if manager.GetOverallHealth() != DegradationNone {
			t.Errorf("Expected overall health NONE initially")
		}

		// Degrade one component
		manager.RecordError(ComponentFileSystemMonitoring, NewFileSystemError("TEST", "Test error", nil))

		if manager.GetOverallHealth() != DegradationMinor {
			t.Errorf("Expected overall health MINOR after one component degraded")
		}

		// Degrade another component with higher level
		manager.RecordError(ComponentCacheRefresh, NewCacheError("TEST", "Test error", nil))

		if manager.GetOverallHealth() != DegradationMajor {
			t.Errorf("Expected overall health MAJOR after higher degradation")
		}
	})

	t.Run("ExecuteWithDegradation calls normal function when healthy", func(t *testing.T) {
		manager := NewGracefulDegradationManager()

		rule := DegradationRule{
			Component:        ComponentFileSystemMonitoring,
			ErrorThreshold:   3,
			TimeWindow:       1 * time.Minute,
			DegradationLevel: DegradationMinor,
		}

		manager.RegisterComponent(rule)

		normalCalled := false
		degradedCalled := false

		err := manager.ExecuteWithDegradation(
			ComponentFileSystemMonitoring,
			func() error {
				normalCalled = true
				return nil
			},
			func(level DegradationLevel) error {
				degradedCalled = true
				return nil
			},
		)

		if err != nil {
			t.Errorf("Expected no error from healthy execution, got %v", err)
		}
		if !normalCalled {
			t.Errorf("Expected normal function to be called")
		}
		if degradedCalled {
			t.Errorf("Expected degraded function not to be called")
		}
	})

	t.Run("ExecuteWithDegradation calls degraded function when unhealthy", func(t *testing.T) {
		manager := NewGracefulDegradationManager()

		rule := DegradationRule{
			Component:        ComponentFileSystemMonitoring,
			ErrorThreshold:   1,
			TimeWindow:       1 * time.Minute,
			DegradationLevel: DegradationMinor,
		}

		manager.RegisterComponent(rule)

		// Trigger degradation
		manager.RecordError(ComponentFileSystemMonitoring, NewFileSystemError("TEST", "Test error", nil))

		normalCalled := false
		degradedCalled := false
		var degradationLevel DegradationLevel

		err := manager.ExecuteWithDegradation(
			ComponentFileSystemMonitoring,
			func() error {
				normalCalled = true
				return nil
			},
			func(level DegradationLevel) error {
				degradedCalled = true
				degradationLevel = level
				return nil
			},
		)

		if err != nil {
			t.Errorf("Expected no error from degraded execution, got %v", err)
		}
		if normalCalled {
			t.Errorf("Expected normal function not to be called")
		}
		if !degradedCalled {
			t.Errorf("Expected degraded function to be called")
		}
		if degradationLevel != DegradationMinor {
			t.Errorf("Expected degradation level MINOR, got %s", degradationLevel)
		}
	})

	t.Run("ForceRecovery recovers component immediately", func(t *testing.T) {
		manager := NewGracefulDegradationManager()

		rule := DegradationRule{
			Component:        ComponentCacheRefresh,
			ErrorThreshold:   1,
			TimeWindow:       1 * time.Minute,
			DegradationLevel: DegradationMajor,
		}

		manager.RegisterComponent(rule)

		// Trigger degradation
		manager.RecordError(ComponentCacheRefresh, NewCacheError("TEST", "Test error", nil))

		status, _ := manager.GetComponentStatus(ComponentCacheRefresh)
		if status.DegradationLevel != DegradationMajor {
			t.Errorf("Expected degradation level MAJOR before recovery")
		}

		// Force recovery
		err := manager.ForceRecovery(ComponentCacheRefresh)
		if err != nil {
			t.Errorf("Expected no error from force recovery, got %v", err)
		}

		status, _ = manager.GetComponentStatus(ComponentCacheRefresh)
		if status.DegradationLevel != DegradationNone {
			t.Errorf("Expected degradation level NONE after force recovery, got %s", status.DegradationLevel)
		}
		if !status.IsHealthy {
			t.Errorf("Expected component to be healthy after force recovery")
		}
	})
}

func TestDegradationStateChangeCallback(t *testing.T) {
	t.Run("State change callback is called on degradation", func(t *testing.T) {
		manager := NewGracefulDegradationManager()

		var callbackCalled bool
		var callbackComponent ServiceComponent
		var callbackOldLevel, callbackNewLevel DegradationLevel

		manager.SetStateChangeCallback(func(component ServiceComponent, oldLevel, newLevel DegradationLevel) {
			callbackCalled = true
			callbackComponent = component
			callbackOldLevel = oldLevel
			callbackNewLevel = newLevel
		})

		rule := DegradationRule{
			Component:        ComponentFileSystemMonitoring,
			ErrorThreshold:   1,
			TimeWindow:       1 * time.Minute,
			DegradationLevel: DegradationMinor,
		}

		manager.RegisterComponent(rule)

		// Trigger degradation
		manager.RecordError(ComponentFileSystemMonitoring, NewFileSystemError("TEST", "Test error", nil))

		// Give callback time to execute (it runs in goroutine)
		time.Sleep(10 * time.Millisecond)

		if !callbackCalled {
			t.Errorf("Expected state change callback to be called")
		}
		if callbackComponent != ComponentFileSystemMonitoring {
			t.Errorf("Expected callback component to be FileSystemMonitoring, got %s", callbackComponent)
		}
		if callbackOldLevel != DegradationNone {
			t.Errorf("Expected callback old level to be NONE, got %s", callbackOldLevel)
		}
		if callbackNewLevel != DegradationMinor {
			t.Errorf("Expected callback new level to be MINOR, got %s", callbackNewLevel)
		}
	})
}

func TestCreateDefaultRules(t *testing.T) {
	t.Run("CreateDefaultRules returns expected components", func(t *testing.T) {
		rules := CreateDefaultRules()

		expectedComponents := []ServiceComponent{
			ComponentFileSystemMonitoring,
			ComponentCacheRefresh,
			ComponentDocumentParsing,
			ComponentResourceDiscovery,
		}

		if len(rules) != len(expectedComponents) {
			t.Errorf("Expected %d default rules, got %d", len(expectedComponents), len(rules))
		}

		componentMap := make(map[ServiceComponent]bool)
		for _, rule := range rules {
			componentMap[rule.Component] = true
		}

		for _, component := range expectedComponents {
			if !componentMap[component] {
				t.Errorf("Expected component %s in default rules", component)
			}
		}
	})

	t.Run("Default rules have reasonable thresholds", func(t *testing.T) {
		rules := CreateDefaultRules()

		for _, rule := range rules {
			if rule.ErrorThreshold <= 0 {
				t.Errorf("Expected positive error threshold for %s, got %d", rule.Component, rule.ErrorThreshold)
			}
			if rule.TimeWindow <= 0 {
				t.Errorf("Expected positive time window for %s, got %v", rule.Component, rule.TimeWindow)
			}
			if rule.RecoveryCondition == nil {
				t.Errorf("Expected recovery condition for %s", rule.Component)
			}
		}
	})
}
