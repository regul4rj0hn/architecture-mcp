package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// MCPBridge creates a TCP server that bridges MCP clients to the stdio-based MCP server
type MCPBridge struct {
	port       int
	host       string
	serverPath string
	listener   net.Listener
	sessions   map[string]*MCPSession
	mu         sync.RWMutex
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
}

func main() {
	var (
		port       = flag.Int("port", 8080, "TCP server port")
		host       = flag.String("host", "localhost", "TCP server host")
		serverPath = flag.String("server", "./bin/mcp-server", "Path to MCP server binary")
	)
	flag.Parse()

	bridge := &MCPBridge{
		port:       *port,
		host:       *host,
		serverPath: *serverPath,
		sessions:   make(map[string]*MCPSession),
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
			log.Printf("Bridge server error: %v", err)
			cancel()
		}
	}()

	// Wait for shutdown signal
	select {
	case <-sigChan:
		log.Println("Received shutdown signal, gracefully shutting down...")
	case <-ctx.Done():
		log.Println("Context cancelled, shutting down...")
	}

	// Graceful shutdown
	if err := bridge.Shutdown(); err != nil {
		log.Printf("Shutdown error: %v", err)
	}
}

func (b *MCPBridge) Start(ctx context.Context) error {
	var err error
	b.listener, err = net.Listen("tcp", fmt.Sprintf("%s:%d", b.host, b.port))
	if err != nil {
		return fmt.Errorf("failed to start listener: %v", err)
	}

	log.Printf("MCP Bridge server listening on %s:%d", b.host, b.port)
	log.Printf("Using MCP server binary: %s", b.serverPath)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			conn, err := b.listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					log.Printf("Accept error: %v", err)
					continue
				}
			}

			// Handle each connection in a separate goroutine
			go b.handleConnection(conn)
		}
	}
}

func (b *MCPBridge) Shutdown() error {
	if b.listener != nil {
		b.listener.Close()
	}

	// Close all sessions
	b.mu.Lock()
	defer b.mu.Unlock()

	for _, session := range b.sessions {
		session.Close()
	}

	log.Println("MCP Bridge shutdown completed")
	return nil
}

func (b *MCPBridge) handleConnection(conn net.Conn) {
	sessionId := fmt.Sprintf("session_%d", time.Now().UnixNano())
	log.Printf("New connection from %s (session: %s)", conn.RemoteAddr(), sessionId)

	// Create new MCP session
	session, err := b.createSession(sessionId, conn)
	if err != nil {
		log.Printf("Failed to create session %s: %v", sessionId, err)
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

	log.Printf("Session %s ended", sessionId)
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

	session := &MCPSession{
		id:      id,
		conn:    conn,
		process: cmd,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		done:    make(chan struct{}),
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
		log.Printf("Session %s: Client -> Server: %s", s.id, line)

		// Parse and forward the JSON message
		var message json.RawMessage
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			log.Printf("Session %s: Invalid JSON from client: %v", s.id, err)
			continue
		}

		if err := encoder.Encode(message); err != nil {
			log.Printf("Session %s: Error forwarding to server: %v", s.id, err)
			return
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Session %s: Client read error: %v", s.id, err)
	}
}

func (s *MCPSession) forwardServerToClient() {
	scanner := bufio.NewScanner(s.stdout)

	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("Session %s: Server -> Client: %s", s.id, line)

		// Forward the response to the client
		if _, err := fmt.Fprintf(s.conn, "%s\n", line); err != nil {
			log.Printf("Session %s: Error forwarding to client: %v", s.id, err)
			return
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Session %s: Server read error: %v", s.id, err)
	}
}

func (s *MCPSession) monitorServerErrors() {
	scanner := bufio.NewScanner(s.stderr)

	for scanner.Scan() {
		line := scanner.Text()
		log.Printf("Session %s: Server stderr: %s", s.id, line)
	}
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
			log.Printf("Session %s: Error killing process: %v", s.id, err)
		}
		s.process.Wait() // Clean up zombie process
	}
}
