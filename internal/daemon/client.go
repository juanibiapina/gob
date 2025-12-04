package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// Client represents a client connection to the daemon
type Client struct {
	conn       net.Conn
	socketPath string
}

// NewClient creates a new daemon client
func NewClient() (*Client, error) {
	socketPath, err := GetSocketPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get socket path: %w", err)
	}

	return &Client{
		socketPath: socketPath,
	}, nil
}

// Connect connects to the daemon, auto-starting it if necessary
func (c *Client) Connect() error {
	// Try to connect
	conn, err := net.Dial("unix", c.socketPath)
	if err == nil {
		c.conn = conn
		return nil
	}

	// Connection failed - try to start daemon
	if err := StartDaemon(); err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	// Retry connection with timeout
	for i := 0; i < 20; i++ {
		conn, err := net.Dial("unix", c.socketPath)
		if err == nil {
			c.conn = conn
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("failed to connect to daemon after starting it")
}

// SendRequest sends a request to the daemon and returns the response
func (c *Client) SendRequest(req *Request) (*Response, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("not connected to daemon")
	}

	encoder := json.NewEncoder(c.conn)
	decoder := json.NewDecoder(c.conn)

	// Send request
	if err := encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Decode response
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &resp, nil
}

// Ping sends a ping request to the daemon
func (c *Client) Ping() error {
	req := NewRequest(RequestTypePing)
	resp, err := c.SendRequest(req)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("ping failed: %s", resp.Error)
	}

	return nil
}

// Shutdown sends a shutdown request to the daemon
func (c *Client) Shutdown() error {
	req := NewRequest(RequestTypeShutdown)
	resp, err := c.SendRequest(req)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("shutdown failed: %s", resp.Error)
	}

	return nil
}

// Close closes the connection to the daemon
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
