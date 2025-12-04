package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
)

// Daemon represents the gob daemon server
type Daemon struct {
	listener   net.Listener
	socketPath string
	pidPath    string
	ctx        context.Context
	cancel     context.CancelFunc
}

// New creates a new daemon instance
func New() (*Daemon, error) {
	socketPath, err := GetSocketPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get socket path: %w", err)
	}

	pidPath, err := GetPIDPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get PID path: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Daemon{
		socketPath: socketPath,
		pidPath:    pidPath,
		ctx:        ctx,
		cancel:     cancel,
	}, nil
}

// Start starts the daemon server
func (d *Daemon) Start() error {
	// Ensure runtime directory exists
	if _, err := EnsureRuntimeDir(); err != nil {
		return fmt.Errorf("failed to ensure runtime directory: %w", err)
	}

	// Clean up stale socket if it exists
	if err := d.cleanupStaleSocket(); err != nil {
		return fmt.Errorf("failed to cleanup stale socket: %w", err)
	}

	// Create Unix socket listener
	listener, err := net.Listen("unix", d.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}
	d.listener = listener

	// Set socket permissions to user-only (0600)
	if err := os.Chmod(d.socketPath, 0600); err != nil {
		d.listener.Close()
		os.Remove(d.socketPath)
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	// Write PID file
	if err := d.writePIDFile(); err != nil {
		d.listener.Close()
		os.Remove(d.socketPath)
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Setup signal handling
	d.setupSignalHandling()

	log.Printf("Daemon started, listening on %s\n", d.socketPath)

	// Accept connections
	go d.acceptConnections()

	return nil
}

// Run runs the daemon until shutdown
func (d *Daemon) Run() error {
	if err := d.Start(); err != nil {
		return err
	}

	// Wait for shutdown signal
	<-d.ctx.Done()

	return d.Shutdown()
}

// Shutdown gracefully shuts down the daemon
func (d *Daemon) Shutdown() error {
	log.Println("Shutting down daemon...")

	// Close listener
	if d.listener != nil {
		d.listener.Close()
	}

	// Remove socket
	if err := os.Remove(d.socketPath); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to remove socket: %v\n", err)
	}

	// Remove PID file
	if err := os.Remove(d.pidPath); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: failed to remove PID file: %v\n", err)
	}

	log.Println("Daemon shut down")
	return nil
}

// cleanupStaleSocket removes a stale socket file if it exists
func (d *Daemon) cleanupStaleSocket() error {
	// Check if socket exists
	if _, err := os.Stat(d.socketPath); os.IsNotExist(err) {
		return nil
	}

	// Try to connect to see if it's stale
	conn, err := net.Dial("unix", d.socketPath)
	if err != nil {
		// Socket exists but can't connect - it's stale
		log.Printf("Removing stale socket: %s\n", d.socketPath)
		return os.Remove(d.socketPath)
	}

	// Socket is active
	conn.Close()
	return fmt.Errorf("daemon already running (socket in use: %s)", d.socketPath)
}

// writePIDFile writes the current process PID to the PID file
func (d *Daemon) writePIDFile() error {
	pid := os.Getpid()
	return os.WriteFile(d.pidPath, []byte(strconv.Itoa(pid)), 0644)
}

// setupSignalHandling sets up graceful shutdown on SIGTERM/SIGINT
func (d *Daemon) setupSignalHandling() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, shutting down...\n", sig)
		d.cancel()
	}()
}

// acceptConnections accepts and handles client connections
func (d *Daemon) acceptConnections() {
	for {
		conn, err := d.listener.Accept()
		if err != nil {
			select {
			case <-d.ctx.Done():
				// Daemon is shutting down
				return
			default:
				log.Printf("Error accepting connection: %v\n", err)
				continue
			}
		}

		// Handle connection in a goroutine
		go d.handleConnection(conn)
	}
}

// handleConnection handles a single client connection
func (d *Daemon) handleConnection(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	// Decode request
	var req Request
	if err := decoder.Decode(&req); err != nil {
		log.Printf("Error decoding request: %v\n", err)
		d.sendErrorResponse(encoder, err)
		return
	}

	// Handle request
	resp := d.handleRequest(&req)

	// Send response
	if err := encoder.Encode(resp); err != nil {
		log.Printf("Error encoding response: %v\n", err)
	}
}

// handleRequest dispatches a request to the appropriate handler
func (d *Daemon) handleRequest(req *Request) *Response {
	switch req.Type {
	case RequestTypePing:
		return d.handlePing(req)
	case RequestTypeShutdown:
		return d.handleShutdown(req)
	default:
		return NewErrorResponse(fmt.Errorf("unknown request type: %s", req.Type))
	}
}

// handlePing handles a ping request
func (d *Daemon) handlePing(req *Request) *Response {
	resp := NewSuccessResponse()
	resp.Data["message"] = "pong"
	return resp
}

// handleShutdown handles a shutdown request
func (d *Daemon) handleShutdown(req *Request) *Response {
	// Trigger shutdown
	go func() {
		d.cancel()
	}()

	resp := NewSuccessResponse()
	resp.Data["message"] = "shutting down"
	return resp
}

// sendErrorResponse sends an error response to the client
func (d *Daemon) sendErrorResponse(encoder *json.Encoder, err error) {
	resp := NewErrorResponse(err)
	encoder.Encode(resp)
}
