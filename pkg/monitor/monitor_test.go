package monitor

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"mcp-architecture-service/internal/models"
)

func TestNewFileSystemMonitor(t *testing.T) {
	monitor, err := NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create file system monitor: %v", err)
	}
	defer monitor.StopWatching()

	if monitor.watcher == nil {
		t.Error("Expected watcher to be initialized")
	}
	if monitor.debounceDelay != 500*time.Millisecond {
		t.Errorf("Expected debounce delay to be 500ms, got %v", monitor.debounceDelay)
	}
	if monitor.callbacks == nil {
		t.Error("Expected callbacks slice to be initialized")
	}
}

func TestWatchDirectoryErrors(t *testing.T) {
	monitor, err := NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create file system monitor: %v", err)
	}
	defer monitor.StopWatching()

	// Test watching non-existent directory
	err = monitor.WatchDirectory("/non/existent/path", func(models.FileEvent) {})
	if err == nil {
		t.Error("Expected error when watching non-existent directory")
	}
}

func TestStopWatching(t *testing.T) {
	monitor, err := NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create file system monitor: %v", err)
	}

	err = monitor.StopWatching()
	if err != nil {
		t.Errorf("StopWatching failed: %v", err)
	}

	// Test stopping again (should not error)
	err = monitor.StopWatching()
	if err != nil {
		t.Errorf("Second StopWatching call failed: %v", err)
	}
}

func TestFileSystemMonitorIntegration(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "monitor_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	monitor, err := NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create file system monitor: %v", err)
	}
	defer monitor.StopWatching()

	// Channel to collect events
	eventChan := make(chan models.FileEvent, 10)
	var mu sync.Mutex
	var events []models.FileEvent

	callback := func(event models.FileEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
		select {
		case eventChan <- event:
		default:
		}
	}

	// Start watching
	err = monitor.WatchDirectory(tempDir, callback)
	if err != nil {
		t.Fatalf("Failed to start watching directory: %v", err)
	}

	// Give the monitor time to start
	time.Sleep(100 * time.Millisecond)

	// Test file creation
	testFile := filepath.Join(tempDir, "test.md")
	err = os.WriteFile(testFile, []byte("# Test\n\nContent"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Wait for event with timeout (could be create or modify)
	select {
	case event := <-eventChan:
		if event.Type != "create" && event.Type != "modify" {
			t.Errorf("Expected create or modify event, got %s", event.Type)
		}
		if event.Path != testFile {
			t.Errorf("Expected path %s, got %s", testFile, event.Path)
		}
		if event.IsDir {
			t.Error("Expected IsDir to be false for file")
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for file creation event")
	}

	// Test file modification
	err = os.WriteFile(testFile, []byte("# Modified Test\n\nNew content"), 0644)
	if err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Wait for modify event
	select {
	case event := <-eventChan:
		if event.Type != "modify" {
			t.Errorf("Expected modify event, got %s", event.Type)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for modify event")
	}

	// Test file deletion
	err = os.Remove(testFile)
	if err != nil {
		t.Fatalf("Failed to remove test file: %v", err)
	}

	// Wait for delete event
	select {
	case event := <-eventChan:
		if event.Type != "delete" {
			t.Errorf("Expected delete event, got %s", event.Type)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for delete event")
	}
}

func TestFileSystemMonitorDebouncing(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "monitor_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	monitor, err := NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create file system monitor: %v", err)
	}
	defer monitor.StopWatching()

	// Channel to collect events
	eventChan := make(chan models.FileEvent, 10)
	var mu sync.Mutex
	var eventCount int

	callback := func(event models.FileEvent) {
		mu.Lock()
		eventCount++
		mu.Unlock()
		select {
		case eventChan <- event:
		default:
		}
	}

	// Start watching
	err = monitor.WatchDirectory(tempDir, callback)
	if err != nil {
		t.Fatalf("Failed to start watching directory: %v", err)
	}

	// Give the monitor time to start
	time.Sleep(100 * time.Millisecond)

	testFile := filepath.Join(tempDir, "debounce_test.md")

	// Rapidly modify the same file multiple times
	for i := 0; i < 5; i++ {
		content := []byte("# Test " + string(rune('0'+i)) + "\n\nContent")
		err = os.WriteFile(testFile, content, 0644)
		if err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
		time.Sleep(50 * time.Millisecond) // Less than debounce delay
	}

	// Wait for debounced events to settle
	time.Sleep(1 * time.Second)

	mu.Lock()
	finalEventCount := eventCount
	mu.Unlock()

	// Should have fewer events than writes due to debouncing
	if finalEventCount >= 5 {
		t.Errorf("Expected fewer than 5 events due to debouncing, got %d", finalEventCount)
	}
	if finalEventCount == 0 {
		t.Error("Expected at least one event after debouncing")
	}
}

func TestFileSystemMonitorNonMarkdownFiles(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "monitor_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	monitor, err := NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create file system monitor: %v", err)
	}
	defer monitor.StopWatching()

	// Channel to collect events
	eventChan := make(chan models.FileEvent, 10)
	var mu sync.Mutex
	var events []models.FileEvent

	callback := func(event models.FileEvent) {
		mu.Lock()
		events = append(events, event)
		mu.Unlock()
		select {
		case eventChan <- event:
		default:
		}
	}

	// Start watching
	err = monitor.WatchDirectory(tempDir, callback)
	if err != nil {
		t.Fatalf("Failed to start watching directory: %v", err)
	}

	// Give the monitor time to start
	time.Sleep(100 * time.Millisecond)

	// Create non-markdown files (should be ignored)
	txtFile := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(txtFile, []byte("Text content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create txt file: %v", err)
	}

	jsonFile := filepath.Join(tempDir, "test.json")
	err = os.WriteFile(jsonFile, []byte(`{"key": "value"}`), 0644)
	if err != nil {
		t.Fatalf("Failed to create json file: %v", err)
	}

	// Create markdown file (should trigger event)
	mdFile := filepath.Join(tempDir, "test.md")
	err = os.WriteFile(mdFile, []byte("# Test\n\nContent"), 0644)
	if err != nil {
		t.Fatalf("Failed to create md file: %v", err)
	}

	// Wait for potential events
	time.Sleep(1 * time.Second)

	mu.Lock()
	eventCount := len(events)
	mu.Unlock()

	// Should only have one event for the markdown file
	if eventCount != 1 {
		t.Errorf("Expected 1 event for markdown file only, got %d", eventCount)
	}

	// Verify the event is for the markdown file
	select {
	case event := <-eventChan:
		if event.Path != mdFile {
			t.Errorf("Expected event for %s, got %s", mdFile, event.Path)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected at least one event for markdown file")
	}
}

func TestMultipleCallbacks(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "monitor_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	monitor, err := NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create file system monitor: %v", err)
	}
	defer monitor.StopWatching()

	// Multiple callback counters
	var callback1Count, callback2Count int
	var mu sync.Mutex

	callback1 := func(event models.FileEvent) {
		mu.Lock()
		callback1Count++
		mu.Unlock()
	}

	callback2 := func(event models.FileEvent) {
		mu.Lock()
		callback2Count++
		mu.Unlock()
	}

	// Register multiple callbacks
	err = monitor.WatchDirectory(tempDir, callback1)
	if err != nil {
		t.Fatalf("Failed to register first callback: %v", err)
	}

	err = monitor.WatchDirectory(tempDir, callback2)
	if err != nil {
		t.Fatalf("Failed to register second callback: %v", err)
	}

	// Give the monitor time to start
	time.Sleep(100 * time.Millisecond)

	// Create a file to trigger events
	testFile := filepath.Join(tempDir, "test.md")
	err = os.WriteFile(testFile, []byte("# Test\n\nContent"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Wait for events to be processed
	time.Sleep(1 * time.Second)

	mu.Lock()
	count1 := callback1Count
	count2 := callback2Count
	mu.Unlock()

	// Both callbacks should have been called at least once (may get multiple events)
	if count1 < 1 {
		t.Errorf("Expected callback1 to be called at least once, got %d", count1)
	}
	if count2 < 1 {
		t.Errorf("Expected callback2 to be called at least once, got %d", count2)
	}
	// Both should have the same count since they receive the same events
	if count1 != count2 {
		t.Errorf("Expected both callbacks to have same count, got %d and %d", count1, count2)
	}
}
