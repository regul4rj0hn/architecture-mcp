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

// setupEventCollection initializes event channels and callbacks for testing
func setupEventCollection(t *testing.T) (chan models.FileEvent, *sync.Mutex, *[]models.FileEvent, func(models.FileEvent)) {
	t.Helper()
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

	return eventChan, &mu, &events, callback
}

// validateFileEvent validates a file event against expected values
func validateFileEvent(t *testing.T, event models.FileEvent, expectedTypes []string, expectedPath string, expectIsDir bool) {
	t.Helper()

	typeMatched := false
	for _, expectedType := range expectedTypes {
		if event.Type == expectedType {
			typeMatched = true
			break
		}
	}
	if !typeMatched {
		t.Errorf("Expected event type to be one of %v, got %s", expectedTypes, event.Type)
	}

	if event.Path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, event.Path)
	}
	if event.IsDir != expectIsDir {
		t.Errorf("Expected IsDir to be %v, got %v", expectIsDir, event.IsDir)
	}
}

// setupMonitorWithTempDir creates a monitor and temp directory for testing
func setupMonitorWithTempDir(t *testing.T) (string, *FileSystemMonitor) {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "monitor_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	monitor, err := NewFileSystemMonitor()
	if err != nil {
		t.Fatalf("Failed to create file system monitor: %v", err)
	}
	t.Cleanup(func() { monitor.StopWatching() })

	return tempDir, monitor
}

// testFileCreationEvent tests file creation event handling
func testFileCreationEvent(t *testing.T, tempDir string, eventChan chan models.FileEvent) string {
	t.Helper()

	testFile := filepath.Join(tempDir, "test.md")
	err := os.WriteFile(testFile, []byte("# Test\n\nContent"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Wait for event with timeout (could be create or modify)
	select {
	case event := <-eventChan:
		validateFileEvent(t, event, []string{"create", "modify"}, testFile, false)
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for file creation event")
	}

	return testFile
}

// testFileModificationEvent tests file modification event handling
func testFileModificationEvent(t *testing.T, testFile string, eventChan chan models.FileEvent) {
	t.Helper()

	err := os.WriteFile(testFile, []byte("# Modified Test\n\nNew content"), 0644)
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
}

// testFileDeletionEvent tests file deletion event handling
func testFileDeletionEvent(t *testing.T, testFile string, eventChan chan models.FileEvent) {
	t.Helper()

	err := os.Remove(testFile)
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

func TestFileSystemMonitorIntegration(t *testing.T) {
	tempDir, monitor := setupMonitorWithTempDir(t)
	eventChan, _, _, callback := setupEventCollection(t)

	err := monitor.WatchDirectory(tempDir, callback)
	if err != nil {
		t.Fatalf("Failed to start watching directory: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	testFile := testFileCreationEvent(t, tempDir, eventChan)
	testFileModificationEvent(t, testFile, eventChan)
	testFileDeletionEvent(t, testFile, eventChan)
}

func TestFileSystemMonitorDebouncing(t *testing.T) {
	tempDir, monitor := setupMonitorWithTempDir(t)
	_, mu, events, callback := setupEventCollection(t)

	err := monitor.WatchDirectory(tempDir, callback)
	if err != nil {
		t.Fatalf("Failed to start watching directory: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	testFile := filepath.Join(tempDir, "debounce_test.md")

	// Rapidly modify the same file multiple times
	for i := range 5 {
		content := []byte("# Test " + string(rune('0'+i)) + "\n\nContent")
		err = os.WriteFile(testFile, content, 0644)
		if err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}

	time.Sleep(1 * time.Second)

	mu.Lock()
	finalEventCount := len(*events)
	mu.Unlock()

	if finalEventCount >= 5 {
		t.Errorf("Expected fewer than 5 events due to debouncing, got %d", finalEventCount)
	}
	if finalEventCount == 0 {
		t.Error("Expected at least one event after debouncing")
	}
}

func TestFileSystemMonitorNonMarkdownFiles(t *testing.T) {
	tempDir, monitor := setupMonitorWithTempDir(t)
	eventChan, mu, events, callback := setupEventCollection(t)

	err := monitor.WatchDirectory(tempDir, callback)
	if err != nil {
		t.Fatalf("Failed to start watching directory: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	tests := []struct {
		name          string
		filename      string
		content       []byte
		shouldTrigger bool
	}{
		{"text file", "test.txt", []byte("Text content"), false},
		{"json file", "test.json", []byte(`{"key": "value"}`), false},
		{"markdown file", "test.md", []byte("# Test\n\nContent"), true},
	}

	var expectedPath string
	for _, tt := range tests {
		filePath := filepath.Join(tempDir, tt.filename)
		err := os.WriteFile(filePath, tt.content, 0644)
		if err != nil {
			t.Fatalf("Failed to create %s: %v", tt.name, err)
		}
		if tt.shouldTrigger {
			expectedPath = filePath
		}
	}

	time.Sleep(1 * time.Second)

	mu.Lock()
	eventCount := len(*events)
	mu.Unlock()

	if eventCount != 1 {
		t.Errorf("Expected 1 event for markdown file only, got %d", eventCount)
	}

	select {
	case event := <-eventChan:
		if event.Path != expectedPath {
			t.Errorf("Expected event for %s, got %s", expectedPath, event.Path)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected at least one event for markdown file")
	}
}

func TestMultipleCallbacks(t *testing.T) {
	tempDir, monitor := setupMonitorWithTempDir(t)

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

	err := monitor.WatchDirectory(tempDir, callback1)
	if err != nil {
		t.Fatalf("Failed to register first callback: %v", err)
	}

	err = monitor.WatchDirectory(tempDir, callback2)
	if err != nil {
		t.Fatalf("Failed to register second callback: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	testFile := filepath.Join(tempDir, "test.md")
	err = os.WriteFile(testFile, []byte("# Test\n\nContent"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	time.Sleep(1 * time.Second)

	mu.Lock()
	count1 := callback1Count
	count2 := callback2Count
	mu.Unlock()

	if count1 < 1 {
		t.Errorf("Expected callback1 to be called at least once, got %d", count1)
	}
	if count2 < 1 {
		t.Errorf("Expected callback2 to be called at least once, got %d", count2)
	}
	if count1 != count2 {
		t.Errorf("Expected both callbacks to have same count, got %d and %d", count1, count2)
	}
}
