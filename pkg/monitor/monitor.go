package monitor

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	"mcp-architecture-service/internal/models"

	"github.com/fsnotify/fsnotify"
)

// FileSystemMonitor monitors file system changes in documentation directories
type FileSystemMonitor struct {
	watcher       *fsnotify.Watcher
	debounceDelay time.Duration
	callbacks     []func(models.FileEvent)
}

// NewFileSystemMonitor creates a new file system monitor
func NewFileSystemMonitor() (*FileSystemMonitor, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &FileSystemMonitor{
		watcher:       watcher,
		debounceDelay: 500 * time.Millisecond, // 500ms debounce
		callbacks:     make([]func(models.FileEvent), 0),
	}, nil
}

// WatchDirectory starts watching a directory for changes
func (fsm *FileSystemMonitor) WatchDirectory(path string, callback func(models.FileEvent)) error {
	// Add callback to list
	fsm.callbacks = append(fsm.callbacks, callback)

	// Add directory to watcher
	err := fsm.watcher.Add(path)
	if err != nil {
		return fmt.Errorf("failed to watch directory %s: %w", path, err)
	}

	// Start monitoring in a goroutine
	go fsm.monitorEvents()

	log.Printf("Started monitoring directory: %s", path)
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
	debounceTimer := make(map[string]*time.Timer)

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
			if timer, exists := debounceTimer[event.Name]; exists {
				timer.Stop()
			}

			debounceTimer[event.Name] = time.AfterFunc(fsm.debounceDelay, func() {
				fsm.processEvent(event)
				delete(debounceTimer, event.Name)
			})

		case err, ok := <-fsm.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("File watcher error: %v", err)
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

	log.Printf("File system event: %s %s", eventType, event.Name)
}
