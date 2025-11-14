package monitor

import (
	"path/filepath"
	"sync"
	"time"

	"mcp-architecture-service/internal/models"
	"mcp-architecture-service/pkg/errors"
	"mcp-architecture-service/pkg/logging"

	"github.com/fsnotify/fsnotify"
)

// FileSystemMonitor monitors file system changes in documentation directories
type FileSystemMonitor struct {
	watcher        *fsnotify.Watcher
	debounceDelay  time.Duration
	callbacks      []func(models.FileEvent)
	logger         *logging.StructuredLogger
	debounceTimers map[string]*time.Timer
	mu             sync.Mutex
}

// NewFileSystemMonitor creates a new file system monitor
func NewFileSystemMonitor() (*FileSystemMonitor, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, errors.NewSystemError(errors.ErrCodeInitializationFailed,
			"Failed to create file watcher", err)
	}

	// Create a default logger for the monitor
	loggingManager := logging.NewLoggingManager()
	logger := loggingManager.GetLogger("file_monitor")

	return &FileSystemMonitor{
		watcher:        watcher,
		debounceDelay:  500 * time.Millisecond, // 500ms debounce
		callbacks:      make([]func(models.FileEvent), 0),
		logger:         logger,
		debounceTimers: make(map[string]*time.Timer),
	}, nil
}

// WatchDirectory starts watching a directory for changes
func (fsm *FileSystemMonitor) WatchDirectory(path string, callback func(models.FileEvent)) error {
	// Add callback to list
	fsm.callbacks = append(fsm.callbacks, callback)

	// Add directory to watcher
	err := fsm.watcher.Add(path)
	if err != nil {
		return errors.NewFileSystemError(errors.ErrCodeFileSystemUnavailable,
			"Failed to watch directory", err).WithContext("path", path)
	}

	// Start monitoring in a goroutine
	go fsm.monitorEvents()

	fsm.logger.WithContext("directory", path).Info("Started monitoring directory")
	return nil
}

// StopWatching stops the file system monitoring
func (fsm *FileSystemMonitor) StopWatching() error {
	if fsm.watcher != nil {
		return fsm.watcher.Close()
	}
	return nil
}

// monitorEvents processes file system events with debouncing
func (fsm *FileSystemMonitor) monitorEvents() {
	for {
		select {
		case event, ok := <-fsm.watcher.Events:
			if !ok {
				return
			}

			// Only process markdown files
			if filepath.Ext(event.Name) != ".md" {
				continue
			}

			// Debounce events for the same file
			fsm.mu.Lock()
			if timer, exists := fsm.debounceTimers[event.Name]; exists {
				timer.Stop()
			}

			fsm.debounceTimers[event.Name] = time.AfterFunc(fsm.debounceDelay, func() {
				fsm.processEvent(event)
				fsm.mu.Lock()
				delete(fsm.debounceTimers, event.Name)
				fsm.mu.Unlock()
			})
			fsm.mu.Unlock()

		case err, ok := <-fsm.watcher.Errors:
			if !ok {
				return
			}
			fsm.logger.WithError(err).Error("File watcher error")
		}
	}
}

// processEvent converts fsnotify events to FileEvent and calls callbacks
func (fsm *FileSystemMonitor) processEvent(event fsnotify.Event) {
	var eventType string
	switch {
	case event.Op&fsnotify.Create == fsnotify.Create:
		eventType = "create"
	case event.Op&fsnotify.Write == fsnotify.Write:
		eventType = "modify"
	case event.Op&fsnotify.Remove == fsnotify.Remove:
		eventType = "delete"
	case event.Op&fsnotify.Rename == fsnotify.Rename:
		eventType = "delete" // Treat rename as delete for simplicity
	default:
		return // Ignore other event types
	}

	fileEvent := models.FileEvent{
		Type:  eventType,
		Path:  event.Name,
		IsDir: false, // We only process files
	}

	// Call all registered callbacks
	for _, callback := range fsm.callbacks {
		callback(fileEvent)
	}

	fsm.logger.WithContext("event_type", eventType).
		WithContext("file_path", event.Name).
		Info("File system event")
}
