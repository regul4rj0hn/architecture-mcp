package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"mcp-architecture-service/pkg/logging"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// MCPBridge creates a TCP server that bridges MCP clients to the stdio-based MCP server
type MCPBridge struct {
	port         int
	host         string
	serverPath   string
	listener     net.Listener
	sessions     map[string]*MCPSession
	mu           sync.RWMutex
	shutdownFlag atomic.Bool
	logger       *logging.StructuredLogger
}

// MCPSession represents a client session with its own MCP server process
type MCPSession struct {
	id      string
	conn    net.Conn
	process *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	stderr  io.ReadCloser
	done    chan struct{}
	mu      sync.Mutex
	logger  *logging.StructuredLogger
}

func main() {
	var (
		port       = flag.Int("port", 8080, "TCP server port")
		host       = flag.String("host", "localhost", "TCP server host")
		serverPath = flag.String("server", "./bin/mcp-server", "Path to MCP server binary")
		logLevel   = flag.String("log-level", "INFO", "Logging level (DEBUG, INFO, WARN, ERROR)")
	)
	flag.Parse()

	// Initialize logging system
	loggingManager := logging.NewLoggingManager()
	loggingManager.SetGlobalContext("service", "mcp-bridge")
	loggingManager.SetGlobalContext("version", "1.0.0")
	loggingManager.SetLogLevel(*logLevel)
	logger := loggingManager.GetLogger("main")

	logger.WithContext("port", *port).
		WithContext("host", *host).
		WithContext("server_path", *serverPath).
		Info("Starting MCP Bridge")

	bridge := &MCPBridge{
		port:       *port,
		host:       *host,
		serverPath: *serverPath,
		sessions:   make(map[string]*MCPSession),
		logger:     loggingManager.GetLogger("bridge"),
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the bridge server
	go func() {
		if err := bridge.Start(ctx); err != nil {
			logger.WithError(err).Error("Bridge server error")
			cancel()
		}
	}()

	// Wait for shutdown signal
	select {
	case <-sigChan:
		logger.Info("Received shutdown signal, gracefully shutting down")
	case <-ctx.Done():
		logger.Info("Context cancelled, shutting down")
	}

	// Graceful shutdown
	if err := bridge.Shutdown(); err != nil {
		logger.WithError(err).Error("Shutdown error")
	}
}

func (b *MCPBridge) Start(ctx context.Context) error {
	var err error
	b.listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", b.host, b.port))
	if err != nil {
		return fmt.Errorf("failed to start listener: %v", err)
	}

	b.logger.WithContext("host", b.host).
		WithContext("port", b.port).
		Info("MCP Bridge server listening")

	b.logger.WithContext("server_path", b.serverPath).
		Info("Using MCP server binary")

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			conn, err := b.listener.Accept()
			if err != nil {
				// Check if we're shutting down
				if b.shutdownFlag.Load() {
					return nil
				}

				// Check context again
				select {
				case <-ctx.Done():
					return nil
				default:
					b.logger.WithError(err).Warn("Accept error")
					continue
				}
			}

			// Handle each connection in a separate goroutine
			go b.handleConnection(conn)
		}
	}
}

func (b *MCPBridge) Shutdown() error {
	// Set shutdown flag BEFORE closing listener
	b.shutdownFlag.Store(true)

	if b.listener != nil {
		b.listener.Close()
	}

	// Close all sessions
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, session := range b.sessions {
		session.Close()
	}

	b.logger.Info("MCP Bridge shutdown completed")
	return nil
}

func (b *MCPBridge) handleConnection(conn net.Conn) {
	sessionId := fmt.Sprintf("session_%d", time.Now().UnixNano())

	b.logger.WithContext("session_id", sessionId).
		WithContext("remote_addr", conn.RemoteAddr().String()).
		Info("New connection")

	// Create new MCP session
	session, err := b.createSession(sessionId, conn)
	if err != nil {
		b.logger.WithError(err).
			WithContext("session_id", sessionId).
			Error("Failed to create session")
		conn.Close()
		return
	}

	// Store session
	b.mu.Lock()
	b.sessions[sessionId] = session
	b.mu.Unlock()

	// Handle the session
	session.Handle()

	// Cleanup
	b.mu.Lock()
	delete(b.sessions, sessionId)
	b.mu.Unlock()

	b.logger.WithContext("session_id", sessionId).Info("Session ended")
}

func (b *MCPBridge) createSession(id string, conn net.Conn) (*MCPSession, error) {
	// Start MCP server process
	cmd := exec.Command(b.serverPath)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("failed to start MCP server: %v", err)
	}

	// Create session logger with session context
	sessionLogger := b.logger.WithContext("session_id", id).
		WithContext("remote_addr", conn.RemoteAddr().String())

	session := &MCPSession{
		id:      id,
		conn:    conn,
		process: cmd,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		done:    make(chan struct{}),
		logger:  sessionLogger,
	}

	return session, nil
}

func (s *MCPSession) Handle() {
	defer s.Close()

	// Start goroutines to handle bidirectional communication
	var wg sync.WaitGroup

	// Client -> MCP Server
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.forwardClientToServer()
	}()

	// MCP Server -> Client
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.forwardServerToClient()
	}()

	// Monitor stderr for debugging
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.monitorServerErrors()
	}()

	// Wait for all goroutines to complete
	wg.Wait()
}

func (s *MCPSession) forwardClientToServer() {
	scanner := bufio.NewScanner(s.conn)
	encoder := json.NewEncoder(s.stdin)

	for scanner.Scan() {
		line := scanner.Text()
		s.logger.WithContext("direction", "client_to_server").
			WithContext("message", line).
			Debug("Forwarding message")

		// Parse and forward the JSON message
		var message json.RawMessage
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			s.logger.WithError(err).
				WithContext("direction", "client_to_server").
				Warn("Invalid JSON from client")
			continue
		}

		if err := encoder.Encode(message); err != nil {
			s.logger.WithError(err).
				WithContext("direction", "client_to_server").
				Error("Error forwarding to server")
			return
		}
	}

	if err := scanner.Err(); err != nil {
		s.logger.WithError(err).
			WithContext("direction", "client_to_server").
			Error("Client read error")
	}
}

func (s *MCPSession) forwardServerToClient() {
	scanner := bufio.NewScanner(s.stdout)

	for scanner.Scan() {
		line := scanner.Text()
		s.logger.WithContext("direction", "server_to_client").
			WithContext("message", line).
			Debug("Forwarding message")

		// Forward the response to the client
		if _, err := fmt.Fprintf(s.conn, "%s\n", line); err != nil {
			s.logger.WithError(err).
				WithContext("direction", "server_to_client").
				Error("Error forwarding to client")
			return
		}
	}

	if err := scanner.Err(); err != nil {
		s.logger.WithError(err).
			WithContext("direction", "server_to_client").
			Error("Server read error")
	}
}

func (s *MCPSession) monitorServerErrors() {
	// Simply copy MCP server stderr to bridge stderr
	// This preserves the original JSON logs without re-parsing
	io.Copy(os.Stderr, s.stderr)
}

func (s *MCPSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case <-s.done:
		return // Already closed
	default:
		close(s.done)
	}

	// Close connection
	if s.conn != nil {
		s.conn.Close()
	}

	// Close pipes
	if s.stdin != nil {
		s.stdin.Close()
	}
	if s.stdout != nil {
		s.stdout.Close()
	}
	if s.stderr != nil {
		s.stderr.Close()
	}

	// Terminate process
	if s.process != nil {
		if err := s.process.Process.Kill(); err != nil {
			s.logger.WithError(err).Error("Error killing process")
		}
		s.process.Wait() // Clean up zombie process
	}
}
