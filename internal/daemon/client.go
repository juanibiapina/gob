package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"syscall"
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

// List returns all jobs, optionally filtered by workdir
func (c *Client) List(workdir string) ([]JobResponse, error) {
	req := NewRequest(RequestTypeList)
	if workdir != "" {
		req.Payload["workdir"] = workdir
	}

	resp, err := c.SendRequest(req)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("list failed: %s", resp.Error)
	}

	// Parse jobs from response
	jobsRaw, ok := resp.Data["jobs"]
	if !ok {
		return []JobResponse{}, nil
	}

	// Convert to JobResponse slice
	jobsJSON, err := json.Marshal(jobsRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal jobs: %w", err)
	}

	var jobs []JobResponse
	if err := json.Unmarshal(jobsJSON, &jobs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal jobs: %w", err)
	}

	return jobs, nil
}

// Add creates and starts a new job
func (c *Client) Add(command []string, workdir string) (*JobResponse, error) {
	req := NewRequest(RequestTypeAdd)
	req.Payload["command"] = command
	req.Payload["workdir"] = workdir

	resp, err := c.SendRequest(req)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("add failed: %s", resp.Error)
	}

	// Parse job from response
	jobRaw, ok := resp.Data["job"]
	if !ok {
		return nil, fmt.Errorf("no job in response")
	}

	jobJSON, err := json.Marshal(jobRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job: %w", err)
	}

	var job JobResponse
	if err := json.Unmarshal(jobJSON, &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, nil
}

// Stop stops a running job
func (c *Client) Stop(jobID string, force bool) (int, error) {
	req := NewRequest(RequestTypeStop)
	req.Payload["job_id"] = jobID
	req.Payload["force"] = force

	resp, err := c.SendRequest(req)
	if err != nil {
		return 0, err
	}

	if !resp.Success {
		return 0, fmt.Errorf("%s", resp.Error)
	}

	pid, _ := resp.Data["pid"].(float64)
	return int(pid), nil
}

// Start starts a stopped job
func (c *Client) Start(jobID string) (*JobResponse, error) {
	req := NewRequest(RequestTypeStart)
	req.Payload["job_id"] = jobID

	resp, err := c.SendRequest(req)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}

	// Parse job from response
	jobRaw, ok := resp.Data["job"]
	if !ok {
		return nil, fmt.Errorf("no job in response")
	}

	jobJSON, err := json.Marshal(jobRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job: %w", err)
	}

	var job JobResponse
	if err := json.Unmarshal(jobJSON, &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, nil
}

// Restart restarts a job
func (c *Client) Restart(jobID string) (*JobResponse, error) {
	req := NewRequest(RequestTypeRestart)
	req.Payload["job_id"] = jobID

	resp, err := c.SendRequest(req)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}

	// Parse job from response
	jobRaw, ok := resp.Data["job"]
	if !ok {
		return nil, fmt.Errorf("no job in response")
	}

	jobJSON, err := json.Marshal(jobRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job: %w", err)
	}

	var job JobResponse
	if err := json.Unmarshal(jobJSON, &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, nil
}

// Remove removes a stopped job
func (c *Client) Remove(jobID string) (int, error) {
	req := NewRequest(RequestTypeRemove)
	req.Payload["job_id"] = jobID

	resp, err := c.SendRequest(req)
	if err != nil {
		return 0, err
	}

	if !resp.Success {
		return 0, fmt.Errorf("%s", resp.Error)
	}

	pid, _ := resp.Data["pid"].(float64)
	return int(pid), nil
}

// Cleanup removes all stopped jobs
func (c *Client) Cleanup(workdir string) (int, error) {
	req := NewRequest(RequestTypeCleanup)
	if workdir != "" {
		req.Payload["workdir"] = workdir
	}

	resp, err := c.SendRequest(req)
	if err != nil {
		return 0, err
	}

	if !resp.Success {
		return 0, fmt.Errorf("cleanup failed: %s", resp.Error)
	}

	count, _ := resp.Data["count"].(float64)
	return int(count), nil
}

// Nuke stops all jobs and removes all data
func (c *Client) Nuke(workdir string) (stopped, logsDeleted, cleaned int, err error) {
	req := NewRequest(RequestTypeNuke)
	if workdir != "" {
		req.Payload["workdir"] = workdir
	}

	resp, err := c.SendRequest(req)
	if err != nil {
		return 0, 0, 0, err
	}

	if !resp.Success {
		return 0, 0, 0, fmt.Errorf("nuke failed: %s", resp.Error)
	}

	stoppedF, _ := resp.Data["stopped"].(float64)
	logsDeletedF, _ := resp.Data["logs_deleted"].(float64)
	cleanedF, _ := resp.Data["cleaned"].(float64)
	return int(stoppedF), int(logsDeletedF), int(cleanedF), nil
}

// Signal sends a signal to a job
func (c *Client) Signal(jobID string, signal syscall.Signal) (int, error) {
	req := NewRequest(RequestTypeSignal)
	req.Payload["job_id"] = jobID
	req.Payload["signal"] = int(signal)

	resp, err := c.SendRequest(req)
	if err != nil {
		return 0, err
	}

	if !resp.Success {
		return 0, fmt.Errorf("%s", resp.Error)
	}

	pid, _ := resp.Data["pid"].(float64)
	return int(pid), nil
}

// GetJob returns a job by ID
func (c *Client) GetJob(jobID string) (*JobResponse, error) {
	req := NewRequest(RequestTypeGetJob)
	req.Payload["job_id"] = jobID

	resp, err := c.SendRequest(req)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}

	// Parse job from response
	jobRaw, ok := resp.Data["job"]
	if !ok {
		return nil, fmt.Errorf("no job in response")
	}

	jobJSON, err := json.Marshal(jobRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job: %w", err)
	}

	var job JobResponse
	if err := json.Unmarshal(jobJSON, &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	return &job, nil
}

// RunResult contains the result of a Run request
type RunResult struct {
	Job       *JobResponse
	Restarted bool
}

// Run finds or creates a job and starts it
func (c *Client) Run(command []string, workdir string) (*RunResult, error) {
	req := NewRequest(RequestTypeRun)
	req.Payload["command"] = command
	req.Payload["workdir"] = workdir

	resp, err := c.SendRequest(req)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}

	// Parse job from response
	jobRaw, ok := resp.Data["job"]
	if !ok {
		return nil, fmt.Errorf("no job in response")
	}

	jobJSON, err := json.Marshal(jobRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job: %w", err)
	}

	var job JobResponse
	if err := json.Unmarshal(jobJSON, &job); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job: %w", err)
	}

	restarted, _ := resp.Data["restarted"].(bool)

	return &RunResult{
		Job:       &job,
		Restarted: restarted,
	}, nil
}

// Close closes the connection to the daemon
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Subscribe subscribes to daemon events and calls the callback for each event
// This blocks until an error occurs or the connection is closed
func (c *Client) Subscribe(workdir string, callback func(Event) error) error {
	if c.conn == nil {
		return fmt.Errorf("not connected to daemon")
	}

	encoder := json.NewEncoder(c.conn)
	decoder := json.NewDecoder(c.conn)

	// Send subscribe request
	req := NewRequest(RequestTypeSubscribe)
	if workdir != "" {
		req.Payload["workdir"] = workdir
	}

	if err := encoder.Encode(req); err != nil {
		return fmt.Errorf("failed to send subscribe request: %w", err)
	}

	// Read initial response
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		return fmt.Errorf("failed to decode subscribe response: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("subscribe failed: %s", resp.Error)
	}

	// Read events in a loop
	for {
		var event Event
		if err := decoder.Decode(&event); err != nil {
			return fmt.Errorf("failed to decode event: %w", err)
		}

		if err := callback(event); err != nil {
			return err
		}
	}
}

// SubscribeChan subscribes to daemon events and returns channels for events and errors
// The caller should select on both channels and handle events/errors appropriately
// To stop the subscription, close the client connection
func (c *Client) SubscribeChan(workdir string) (<-chan Event, <-chan error) {
	eventCh := make(chan Event, 10)
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)
		defer close(errCh)

		err := c.Subscribe(workdir, func(event Event) error {
			eventCh <- event
			return nil
		})
		if err != nil {
			errCh <- err
		}
	}()

	return eventCh, errCh
}
