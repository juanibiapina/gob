package daemon

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/juanibiapina/gob/internal/version"
)

// ErrOldDaemon is returned when the daemon does not support version negotiation
var ErrOldDaemon = errors.New("daemon does not support version negotiation")

// VersionInfo contains daemon version information
type VersionInfo struct {
	Version     string // Semantic version (e.g., "1.2.3")
	RunningJobs int    // Number of currently running jobs
}

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
	return c.connect(false)
}

// ConnectSkipVersionCheck connects without checking daemon version (used by shutdown)
func (c *Client) ConnectSkipVersionCheck() error {
	return c.connect(true)
}

// connect is the internal connection logic
func (c *Client) connect(skipVersionCheck bool) error {
	// Try to connect
	conn, err := net.Dial("unix", c.socketPath)
	if err == nil {
		c.conn = conn
		// Check daemon version compatibility (unless skipped)
		if skipVersionCheck {
			return nil
		}
		return c.CheckDaemonVersion()
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
			// No need to check version - we just started the daemon
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("failed to connect to daemon after starting it")
}

// SendRequest sends a request to the daemon and returns the response
func (c *Client) SendRequest(req *Request) (*Response, error) {
	// Reconnect for each request (daemon closes connection after each response)
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer conn.Close()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

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

// Add creates and starts a new job with the given environment
func (c *Client) Add(command []string, workdir string, env []string) (*AddResponse, error) {
	req := NewRequest(RequestTypeAdd)
	req.Payload["command"] = command
	req.Payload["workdir"] = workdir
	req.Payload["env"] = env

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

	result := &AddResponse{Job: job}

	// Parse stats if present (job has previous completed runs)
	if statsRaw, ok := resp.Data["stats"]; ok {
		statsJSON, err := json.Marshal(statsRaw)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal stats: %w", err)
		}

		var stats StatsResponse
		if err := json.Unmarshal(statsJSON, &stats); err != nil {
			return nil, fmt.Errorf("failed to unmarshal stats: %w", err)
		}
		result.Stats = &stats
	}

	return result, nil
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

// Start starts a stopped job with the given environment
func (c *Client) Start(jobID string, env []string) (*JobResponse, error) {
	req := NewRequest(RequestTypeStart)
	req.Payload["job_id"] = jobID
	req.Payload["env"] = env

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

// Restart restarts a job with the given environment
func (c *Client) Restart(jobID string, env []string) (*JobResponse, error) {
	req := NewRequest(RequestTypeRestart)
	req.Payload["job_id"] = jobID
	req.Payload["env"] = env

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

// StopAll stops all running jobs
func (c *Client) StopAll() (stopped int, err error) {
	req := NewRequest(RequestTypeStopAll)

	resp, err := c.SendRequest(req)
	if err != nil {
		return 0, err
	}

	if !resp.Success {
		return 0, fmt.Errorf("stop_all failed: %s", resp.Error)
	}

	stoppedF, _ := resp.Data["stopped"].(float64)
	return int(stoppedF), nil
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

// Runs returns the run history for a job
func (c *Client) Runs(jobID string) ([]RunResponse, error) {
	req := NewRequest(RequestTypeRuns)
	req.Payload["job_id"] = jobID

	resp, err := c.SendRequest(req)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}

	// Parse runs from response
	runsRaw, ok := resp.Data["runs"]
	if !ok {
		return []RunResponse{}, nil
	}

	runsJSON, err := json.Marshal(runsRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal runs: %w", err)
	}

	var runs []RunResponse
	if err := json.Unmarshal(runsJSON, &runs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal runs: %w", err)
	}

	return runs, nil
}

// Stats returns statistics for a job
func (c *Client) Stats(jobID string) (*StatsResponse, error) {
	req := NewRequest(RequestTypeStats)
	req.Payload["job_id"] = jobID

	resp, err := c.SendRequest(req)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}

	// Parse stats from response
	statsRaw, ok := resp.Data["stats"]
	if !ok {
		return nil, fmt.Errorf("no stats in response")
	}

	statsJSON, err := json.Marshal(statsRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal stats: %w", err)
	}

	var stats StatsResponse
	if err := json.Unmarshal(statsJSON, &stats); err != nil {
		return nil, fmt.Errorf("failed to unmarshal stats: %w", err)
	}

	return &stats, nil
}

// Ports returns the listening ports for a job
func (c *Client) Ports(jobID string) (*JobPorts, error) {
	req := NewRequest(RequestTypePorts)
	req.Payload["job_id"] = jobID

	resp, err := c.SendRequest(req)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}

	// Parse ports from response
	portsRaw, ok := resp.Data["ports"]
	if !ok {
		return nil, fmt.Errorf("no ports in response")
	}

	portsJSON, err := json.Marshal(portsRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ports: %w", err)
	}

	var ports JobPorts
	if err := json.Unmarshal(portsJSON, &ports); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ports: %w", err)
	}

	return &ports, nil
}

// AllPorts returns the listening ports for all running jobs
func (c *Client) AllPorts(workdir string) ([]JobPorts, error) {
	req := NewRequest(RequestTypePorts)
	if workdir != "" {
		req.Payload["workdir"] = workdir
	}

	resp, err := c.SendRequest(req)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}

	// Parse ports from response
	portsRaw, ok := resp.Data["ports"]
	if !ok {
		return []JobPorts{}, nil
	}

	portsJSON, err := json.Marshal(portsRaw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ports: %w", err)
	}

	var ports []JobPorts
	if err := json.Unmarshal(portsJSON, &ports); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ports: %w", err)
	}

	return ports, nil
}

// Close closes the connection to the daemon
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// GetDaemonVersion retrieves version information from the daemon
func (c *Client) GetDaemonVersion() (*VersionInfo, error) {
	req := NewRequest(RequestTypeVersion)
	req.Payload["client_version"] = version.Version

	resp, err := c.SendRequest(req)
	if err != nil {
		return nil, err
	}

	// Old daemon returns error for unknown request type
	if !resp.Success {
		if strings.Contains(resp.Error, "unknown request type") {
			return nil, ErrOldDaemon
		}
		return nil, fmt.Errorf("version check failed: %s", resp.Error)
	}

	// Parse version info from daemon
	return &VersionInfo{
		Version:     resp.Data["version"].(string),
		RunningJobs: int(resp.Data["running_jobs"].(float64)),
	}, nil
}

// CheckDaemonVersion checks version compatibility and handles upgrades
func (c *Client) CheckDaemonVersion() error {
	info, err := c.GetDaemonVersion()
	if errors.Is(err, ErrOldDaemon) {
		return c.handleOldDaemon()
	}
	if err != nil {
		return err
	}

	// Version matches - nothing to do
	if info.Version == version.Version {
		return nil
	}

	// Version mismatch - restart if no running jobs, otherwise error
	if info.RunningJobs == 0 {
		return c.restartDaemon(fmt.Sprintf("version mismatch: daemon=%s, client=%s", info.Version, version.Version))
	}

	return fmt.Errorf("daemon version mismatch (daemon=%s, client=%s) but has %d running job(s); run 'gob shutdown' to stop all jobs and restart daemon",
		info.Version, version.Version, info.RunningJobs)
}

// handleOldDaemon handles a daemon that doesn't support version negotiation
func (c *Client) handleOldDaemon() error {
	// Try to get job count via list (old daemons support this)
	jobs, err := c.List("")
	if err != nil {
		// Can't even list - daemon is really broken, just restart it
		return c.restartDaemon("outdated version")
	}

	// Count running jobs
	runningCount := 0
	for _, job := range jobs {
		if job.Status == "running" {
			runningCount++
		}
	}

	if runningCount == 0 {
		// Safe to restart
		return c.restartDaemon("outdated version")
	}

	// Has running jobs - return error with guidance
	return fmt.Errorf("daemon version outdated but has %d running job(s); run 'gob shutdown' to stop all jobs and restart daemon", runningCount)
}

// restartDaemon shuts down the current daemon and starts a new one
func (c *Client) restartDaemon(reason string) error {
	// Send shutdown to old daemon
	c.Shutdown() // Ignore error - daemon may already be gone

	// Wait briefly for socket to be released
	time.Sleep(100 * time.Millisecond)

	// Reconnect will auto-start new daemon
	return c.reconnect()
}

// reconnect establishes a new connection, starting daemon if needed
func (c *Client) reconnect() error {
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
