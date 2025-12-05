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
	runtimeDir string
	ctx        context.Context
	cancel     context.CancelFunc
	jobManager *JobManager
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

	runtimeDir, err := GetRuntimeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime directory: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Daemon{
		socketPath: socketPath,
		pidPath:    pidPath,
		runtimeDir: runtimeDir,
		ctx:        ctx,
		cancel:     cancel,
		jobManager: NewJobManager(runtimeDir),
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

	// Stop all managed jobs first
	stopped, _, _ := d.jobManager.Nuke("")
	if stopped > 0 {
		log.Printf("Stopped %d running job(s)\n", stopped)
	}

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
	case RequestTypeList:
		return d.handleList(req)
	case RequestTypeAdd:
		return d.handleAdd(req)
	case RequestTypeStop:
		return d.handleStop(req)
	case RequestTypeStart:
		return d.handleStart(req)
	case RequestTypeRestart:
		return d.handleRestart(req)
	case RequestTypeRemove:
		return d.handleRemove(req)
	case RequestTypeCleanup:
		return d.handleCleanup(req)
	case RequestTypeNuke:
		return d.handleNuke(req)
	case RequestTypeSignal:
		return d.handleSignal(req)
	case RequestTypeGetJob:
		return d.handleGetJob(req)
	case RequestTypeRun:
		return d.handleRun(req)
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

// handleList handles a list request
func (d *Daemon) handleList(req *Request) *Response {
	workdir, _ := req.Payload["workdir"].(string)
	jobs := d.jobManager.ListJobs(workdir)

	var jobResponses []JobResponse
	for _, job := range jobs {
		jobResponses = append(jobResponses, JobResponse{
			ID:         job.ID,
			PID:        job.PID,
			Status:     job.Status(),
			Command:    job.Command,
			Workdir:    job.Workdir,
			CreatedAt:  job.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			StdoutPath: job.StdoutPath,
			StderrPath: job.StderrPath,
		})
	}

	resp := NewSuccessResponse()
	resp.Data["jobs"] = jobResponses
	return resp
}

// handleAdd handles an add request
func (d *Daemon) handleAdd(req *Request) *Response {
	// Extract command
	commandRaw, ok := req.Payload["command"]
	if !ok {
		return NewErrorResponse(fmt.Errorf("missing command"))
	}

	// Convert to []string
	var command []string
	switch v := commandRaw.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				command = append(command, s)
			}
		}
	default:
		return NewErrorResponse(fmt.Errorf("invalid command format"))
	}

	if len(command) == 0 {
		return NewErrorResponse(fmt.Errorf("empty command"))
	}

	workdir, _ := req.Payload["workdir"].(string)
	if workdir == "" {
		return NewErrorResponse(fmt.Errorf("missing workdir"))
	}

	job, err := d.jobManager.AddJob(command, workdir)
	if err != nil {
		return NewErrorResponse(err)
	}

	resp := NewSuccessResponse()
	resp.Data["job"] = JobResponse{
		ID:         job.ID,
		PID:        job.PID,
		Status:     job.Status(),
		Command:    job.Command,
		Workdir:    job.Workdir,
		CreatedAt:  job.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		StdoutPath: job.StdoutPath,
		StderrPath: job.StderrPath,
	}
	return resp
}

// handleStop handles a stop request
func (d *Daemon) handleStop(req *Request) *Response {
	jobID, ok := req.Payload["job_id"].(string)
	if !ok {
		return NewErrorResponse(fmt.Errorf("missing job_id"))
	}

	force, _ := req.Payload["force"].(bool)

	job, err := d.jobManager.GetJob(jobID)
	if err != nil {
		return NewErrorResponse(err)
	}

	if err := d.jobManager.StopJob(jobID, force); err != nil {
		return NewErrorResponse(err)
	}

	resp := NewSuccessResponse()
	resp.Data["job_id"] = jobID
	resp.Data["pid"] = job.PID
	resp.Data["force"] = force
	return resp
}

// handleStart handles a start request
func (d *Daemon) handleStart(req *Request) *Response {
	jobID, ok := req.Payload["job_id"].(string)
	if !ok {
		return NewErrorResponse(fmt.Errorf("missing job_id"))
	}

	if err := d.jobManager.StartJob(jobID); err != nil {
		return NewErrorResponse(err)
	}

	job, _ := d.jobManager.GetJob(jobID)

	resp := NewSuccessResponse()
	resp.Data["job"] = JobResponse{
		ID:         job.ID,
		PID:        job.PID,
		Status:     job.Status(),
		Command:    job.Command,
		Workdir:    job.Workdir,
		CreatedAt:  job.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		StdoutPath: job.StdoutPath,
		StderrPath: job.StderrPath,
	}
	return resp
}

// handleRestart handles a restart request
func (d *Daemon) handleRestart(req *Request) *Response {
	jobID, ok := req.Payload["job_id"].(string)
	if !ok {
		return NewErrorResponse(fmt.Errorf("missing job_id"))
	}

	if err := d.jobManager.RestartJob(jobID); err != nil {
		return NewErrorResponse(err)
	}

	job, _ := d.jobManager.GetJob(jobID)

	resp := NewSuccessResponse()
	resp.Data["job"] = JobResponse{
		ID:         job.ID,
		PID:        job.PID,
		Status:     job.Status(),
		Command:    job.Command,
		Workdir:    job.Workdir,
		CreatedAt:  job.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		StdoutPath: job.StdoutPath,
		StderrPath: job.StderrPath,
	}
	return resp
}

// handleRemove handles a remove request
func (d *Daemon) handleRemove(req *Request) *Response {
	jobID, ok := req.Payload["job_id"].(string)
	if !ok {
		return NewErrorResponse(fmt.Errorf("missing job_id"))
	}

	job, err := d.jobManager.GetJob(jobID)
	if err != nil {
		return NewErrorResponse(err)
	}
	pid := job.PID

	if err := d.jobManager.RemoveJob(jobID); err != nil {
		return NewErrorResponse(err)
	}

	resp := NewSuccessResponse()
	resp.Data["job_id"] = jobID
	resp.Data["pid"] = pid
	return resp
}

// handleCleanup handles a cleanup request
func (d *Daemon) handleCleanup(req *Request) *Response {
	workdir, _ := req.Payload["workdir"].(string)
	count := d.jobManager.Cleanup(workdir)

	resp := NewSuccessResponse()
	resp.Data["count"] = count
	return resp
}

// handleNuke handles a nuke request
func (d *Daemon) handleNuke(req *Request) *Response {
	workdir, _ := req.Payload["workdir"].(string)
	stopped, logsDeleted, cleaned := d.jobManager.Nuke(workdir)

	resp := NewSuccessResponse()
	resp.Data["stopped"] = stopped
	resp.Data["logs_deleted"] = logsDeleted
	resp.Data["cleaned"] = cleaned
	return resp
}

// handleSignal handles a signal request
func (d *Daemon) handleSignal(req *Request) *Response {
	jobID, ok := req.Payload["job_id"].(string)
	if !ok {
		return NewErrorResponse(fmt.Errorf("missing job_id"))
	}

	signalNum, ok := req.Payload["signal"].(float64)
	if !ok {
		return NewErrorResponse(fmt.Errorf("missing signal"))
	}

	job, err := d.jobManager.GetJob(jobID)
	if err != nil {
		return NewErrorResponse(err)
	}

	if err := d.jobManager.Signal(jobID, syscall.Signal(int(signalNum))); err != nil {
		return NewErrorResponse(err)
	}

	resp := NewSuccessResponse()
	resp.Data["job_id"] = jobID
	resp.Data["pid"] = job.PID
	resp.Data["signal"] = int(signalNum)
	return resp
}

// handleGetJob handles a get_job request
func (d *Daemon) handleGetJob(req *Request) *Response {
	jobID, ok := req.Payload["job_id"].(string)
	if !ok {
		return NewErrorResponse(fmt.Errorf("missing job_id"))
	}

	job, err := d.jobManager.GetJob(jobID)
	if err != nil {
		return NewErrorResponse(err)
	}

	resp := NewSuccessResponse()
	resp.Data["job"] = JobResponse{
		ID:         job.ID,
		PID:        job.PID,
		Status:     job.Status(),
		Command:    job.Command,
		Workdir:    job.Workdir,
		CreatedAt:  job.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		StdoutPath: job.StdoutPath,
		StderrPath: job.StderrPath,
	}
	return resp
}

// handleRun handles a run request (find/reuse job, or create new)
func (d *Daemon) handleRun(req *Request) *Response {
	// Extract command
	commandRaw, ok := req.Payload["command"]
	if !ok {
		return NewErrorResponse(fmt.Errorf("missing command"))
	}

	// Convert to []string
	var command []string
	switch v := commandRaw.(type) {
	case []interface{}:
		for _, item := range v {
			if s, ok := item.(string); ok {
				command = append(command, s)
			}
		}
	default:
		return NewErrorResponse(fmt.Errorf("invalid command format"))
	}

	if len(command) == 0 {
		return NewErrorResponse(fmt.Errorf("empty command"))
	}

	workdir, _ := req.Payload["workdir"].(string)
	if workdir == "" {
		return NewErrorResponse(fmt.Errorf("missing workdir"))
	}

	// Check if job with same command exists
	job, isRestart, err := d.jobManager.RunJob(command, workdir)
	if err != nil {
		return NewErrorResponse(err)
	}

	resp := NewSuccessResponse()
	resp.Data["job"] = JobResponse{
		ID:         job.ID,
		PID:        job.PID,
		Status:     job.Status(),
		Command:    job.Command,
		Workdir:    job.Workdir,
		CreatedAt:  job.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		StdoutPath: job.StdoutPath,
		StderrPath: job.StderrPath,
	}
	resp.Data["restarted"] = isRestart
	return resp
}

// sendErrorResponse sends an error response to the client
func (d *Daemon) sendErrorResponse(encoder *json.Encoder, err error) {
	resp := NewErrorResponse(err)
	encoder.Encode(resp)
}
