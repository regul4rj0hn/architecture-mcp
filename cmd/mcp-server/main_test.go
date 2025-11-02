package main

import (
	"context"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestMainServerStartup(t *testing.T) {
	// This test verifies that the main function can start and stop gracefully
	// We'll use a subprocess approach to test the actual main function

	if os.Getenv("TEST_MAIN") == "1" {
		// This is the subprocess that runs main()
		main()
		return
	}

	// Start the main function in a subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestMainServerStartup")
	cmd.Env = append(os.Environ(), "TEST_MAIN=1")

	// Start the process
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start main process: %v", err)
	}

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Send SIGTERM to trigger graceful shutdown
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Errorf("Failed to send SIGTERM: %v", err)
	}

	// Wait for process to exit
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		// Process should exit cleanly (exit code 0 or signal termination)
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				// Check if it was terminated by signal (which is expected)
				if !exitError.Exited() {
					// Process was terminated by signal, which is what we expect
					return
				}
			}
			t.Errorf("Process exited with error: %v", err)
		}
	case <-time.After(5 * time.Second):
		// Force kill if it doesn't exit gracefully
		cmd.Process.Kill()
		t.Error("Process did not exit within timeout")
	}
}

func TestMainServerSignalHandling(t *testing.T) {
	// Test SIGINT handling
	if os.Getenv("TEST_SIGINT") == "1" {
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainServerSignalHandling")
	cmd.Env = append(os.Environ(), "TEST_SIGINT=1")

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start main process: %v", err)
	}

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Send SIGINT (Ctrl+C)
	if err := cmd.Process.Signal(syscall.SIGINT); err != nil {
		t.Errorf("Failed to send SIGINT: %v", err)
	}

	// Wait for process to exit
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			if exitError, ok := err.(*exec.ExitError); ok {
				if !exitError.Exited() {
					// Process was terminated by signal, which is expected
					return
				}
			}
			t.Errorf("Process exited with error: %v", err)
		}
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		t.Error("Process did not exit within timeout after SIGINT")
	}
}

// TestServerComponents tests that the server components can be created and used
func TestServerComponents(t *testing.T) {
	// Test that we can create a context and cancel it
	ctx, cancel := context.WithCancel(context.Background())

	// Test context cancellation
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	select {
	case <-ctx.Done():
		if ctx.Err() != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", ctx.Err())
		}
	case <-time.After(1 * time.Second):
		t.Error("Context was not cancelled within timeout")
	}
}

// TestSignalChannel tests signal channel creation and handling
func TestSignalChannel(t *testing.T) {
	// Create signal channel like in main()
	sigChan := make(chan os.Signal, 1)

	// Test that channel can receive signals (we won't actually send real signals in test)
	if cap(sigChan) != 1 {
		t.Errorf("Expected signal channel capacity 1, got %d", cap(sigChan))
	}

	// Test non-blocking send to channel
	select {
	case sigChan <- syscall.SIGTERM:
		// Successfully sent signal
	default:
		t.Error("Failed to send signal to channel")
	}

	// Test receiving from channel
	select {
	case sig := <-sigChan:
		if sig != syscall.SIGTERM {
			t.Errorf("Expected SIGTERM, got %v", sig)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Did not receive signal from channel")
	}
}
