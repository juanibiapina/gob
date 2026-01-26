package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/juanibiapina/gob/internal/version"
)

// Subscriber represents a client subscribed to events
type Subscriber struct {
	conn    net.Conn
	encoder *json.Encoder
	workdir string
}

// Daemon represents the gob daemon server
type Daemon struct {
	listener      net.Listener
	socketPath    string
	pidPath       string
	runtimeDir    string
	logDir        string
	db            *sql.DB
	store         *Store
	ctx           context.Context
	cancel        context.CancelFunc
	jobManager    *JobManager
	subscribers   []*Subscriber
	subscribersMu sync.RWMutex
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

	// Ensure state directory exists for database
	if _, err := EnsureStateDir(); err != nil {
		return nil, fmt.Errorf("failed to ensure state directory: %w", err)
	}

	// Ensure log directory exists
	logDir, err := EnsureLogDir()
	if err != nil {
		return nil, fmt.Errorf("failed to ensure log directory: %w", err)
	}

	// Open database
	dbPath, err := GetDatabasePath()
	if err != nil {
		return nil, fmt.Errorf("failed to get database path: %w", err)
	}

	db, err := OpenDatabase(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	store := NewStore(db)

	ctx, cancel := context.WithCancel(context.Background())

	d := &Daemon{
		socketPath:  socketPath,
		pidPath:     pidPath,
		runtimeDir:  runtimeDir,
		logDir:      logDir,
		db:          db,
		store:       store,
		ctx:         ctx,
		cancel:      cancel,
		subscribers: make([]*Subscriber, 0),
	}

	// Initialize job manager with event callback and store
	d.jobManager = NewJobManager(logDir, d.handleEvent, store)

	return d, nil
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

	// Check for crash recovery
	if !d.store.WasCleanShutdown() {
		Logger.Info("previous shutdown was not clean, performing crash recovery")
		if err := d.recoverFromCrash(); err != nil {
			Logger.Error("crash recovery failed", "error", err)
			// Continue anyway - jobs will be loaded but orphans may not be cleaned
		}
	}

	// Set shutdown_clean = false (will be set to true on graceful shutdown)
	if err := d.store.SetShutdownClean(false); err != nil {
		return fmt.Errorf("failed to set shutdown flag: %w", err)
	}

	// Record instance ID
	if err := d.store.SetInstanceID(); err != nil {
		return fmt.Errorf("failed to set instance ID: %w", err)
	}

	// Load jobs and runs from database
	if err := d.jobManager.LoadFromStore(); err != nil {
		return fmt.Errorf("failed to load state from database: %w", err)
	}

	Logger.Info("loaded persisted state", "jobs", d.jobManager.JobCount())

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

	Logger.Info("daemon started", "socket", d.socketPath)

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
	Logger.Info("shutting down daemon")

	// Close all subscriber connections
	d.subscribersMu.Lock()
	for _, sub := range d.subscribers {
		sub.conn.Close()
	}
	d.subscribers = nil
	d.subscribersMu.Unlock()

	// Stop all managed jobs first
	stopped := d.jobManager.StopAll()
	if stopped > 0 {
		Logger.Info("stopped running jobs", "count", stopped)
	}

	// Set shutdown_clean = true since we're shutting down gracefully
	if err := d.store.SetShutdownClean(true); err != nil {
		Logger.Warn("failed to set shutdown_clean flag", "error", err)
	}

	// Close database
	if d.db != nil {
		if err := d.db.Close(); err != nil {
			Logger.Warn("failed to close database", "error", err)
		}
	}

	// Close listener
	if d.listener != nil {
		d.listener.Close()
	}

	// Remove socket
	if err := os.Remove(d.socketPath); err != nil && !os.IsNotExist(err) {
		Logger.Warn("failed to remove socket", "error", err)
	}

	// Remove PID file
	if err := os.Remove(d.pidPath); err != nil && !os.IsNotExist(err) {
		Logger.Warn("failed to remove PID file", "error", err)
	}

	Logger.Info("daemon shut down")
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
		Logger.Info("removing stale socket", "path", d.socketPath)
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
		Logger.Info("received signal", "signal", sig)
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
				Logger.Error("error accepting connection", "error", err)
				continue
			}
		}

		// Handle connection in a goroutine
		go d.handleConnection(conn)
	}
}

// handleConnection handles a single client connection
func (d *Daemon) handleConnection(conn net.Conn) {
	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	// Decode request
	var req Request
	if err := decoder.Decode(&req); err != nil {
		Logger.Error("error decoding request", "error", err)
		d.sendErrorResponse(encoder, err)
		conn.Close()
		return
	}

	// Handle subscribe specially - don't close connection
	if req.Type == RequestTypeSubscribe {
		d.handleSubscribe(&req, conn, encoder)
		return
	}

	// For all other requests, close connection after handling
	defer conn.Close()

	// Handle request
	resp := d.handleRequest(&req)

	// Send response
	if err := encoder.Encode(resp); err != nil {
		Logger.Error("error encoding response", "error", err)
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
	case RequestTypeCreate:
		return d.handleCreate(req)
	case RequestTypeStop:
		return d.handleStop(req)
	case RequestTypeStart:
		return d.handleStart(req)
	case RequestTypeRestart:
		return d.handleRestart(req)
	case RequestTypeRemove:
		return d.handleRemove(req)
	case RequestTypeStopAll:
		return d.handleStopAll(req)
	case RequestTypeSignal:
		return d.handleSignal(req)
	case RequestTypeGetJob:
		return d.handleGetJob(req)
	case RequestTypeVersion:
		return d.handleVersion(req)
	case RequestTypeRuns:
		return d.handleRuns(req)
	case RequestTypeStats:
		return d.handleStats(req)
	case RequestTypePorts:
		return d.handlePorts(req)
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
		jobResponses = append(jobResponses, d.jobManager.jobToResponse(job))
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

	// Extract optional description
	description, _ := req.Payload["description"].(string)

	// Extract environment
	var env []string
	if envRaw, ok := req.Payload["env"]; ok {
		switch v := envRaw.(type) {
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					env = append(env, s)
				}
			}
		}
	}

	job, action, err := d.jobManager.AddJob(command, workdir, description, env)
	if err != nil {
		return NewErrorResponse(err)
	}

	resp := NewSuccessResponse()
	resp.Data["job"] = d.jobManager.jobToResponse(job)
	resp.Data["action"] = action

	// Include stats if job has previous completed runs
	// RunCount is updated on completion, so check if > 0
	if job.RunCount > 0 {
		stats := jobToStats(job)
		resp.Data["stats"] = stats
	}

	return resp
}

// handleCreate handles a create request (add job without starting)
func (d *Daemon) handleCreate(req *Request) *Response {
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

	// Extract optional description
	description, _ := req.Payload["description"].(string)

	job, err := d.jobManager.CreateJob(command, workdir, description)
	if err != nil {
		return NewErrorResponse(err)
	}

	resp := NewSuccessResponse()
	resp.Data["job"] = d.jobManager.jobToResponse(job)

	return resp
}

// handleStop handles a stop request
func (d *Daemon) handleStop(req *Request) *Response {
	jobID, ok := req.Payload["job_id"].(string)
	if !ok {
		return NewErrorResponse(fmt.Errorf("missing job_id"))
	}

	force, _ := req.Payload["force"].(bool)

	// Get PID from current run, or latest run if stopped
	run := d.jobManager.GetCurrentRun(jobID)
	if run == nil {
		run = d.jobManager.GetLatestRun(jobID)
	}
	var pid int
	if run != nil {
		pid = run.PID
	}

	if err := d.jobManager.StopJob(jobID, force); err != nil {
		return NewErrorResponse(err)
	}

	resp := NewSuccessResponse()
	resp.Data["job_id"] = jobID
	resp.Data["pid"] = pid
	resp.Data["force"] = force
	return resp
}

// handleStart handles a start request
func (d *Daemon) handleStart(req *Request) *Response {
	jobID, ok := req.Payload["job_id"].(string)
	if !ok {
		return NewErrorResponse(fmt.Errorf("missing job_id"))
	}

	// Extract environment
	var env []string
	if envRaw, ok := req.Payload["env"]; ok {
		switch v := envRaw.(type) {
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					env = append(env, s)
				}
			}
		}
	}

	if err := d.jobManager.StartJob(jobID, env); err != nil {
		return NewErrorResponse(err)
	}

	job, _ := d.jobManager.GetJob(jobID)

	resp := NewSuccessResponse()
	resp.Data["job"] = d.jobManager.jobToResponse(job)
	return resp
}

// handleRestart handles a restart request
func (d *Daemon) handleRestart(req *Request) *Response {
	jobID, ok := req.Payload["job_id"].(string)
	if !ok {
		return NewErrorResponse(fmt.Errorf("missing job_id"))
	}

	// Extract environment
	var env []string
	if envRaw, ok := req.Payload["env"]; ok {
		switch v := envRaw.(type) {
		case []interface{}:
			for _, item := range v {
				if s, ok := item.(string); ok {
					env = append(env, s)
				}
			}
		}
	}

	if err := d.jobManager.RestartJob(jobID, env); err != nil {
		return NewErrorResponse(err)
	}

	job, _ := d.jobManager.GetJob(jobID)

	resp := NewSuccessResponse()
	resp.Data["job"] = d.jobManager.jobToResponse(job)
	return resp
}

// handleRemove handles a remove request
func (d *Daemon) handleRemove(req *Request) *Response {
	jobID, ok := req.Payload["job_id"].(string)
	if !ok {
		return NewErrorResponse(fmt.Errorf("missing job_id"))
	}

	run := d.jobManager.GetLatestRun(jobID)
	var pid int
	if run != nil {
		pid = run.PID
	}

	if err := d.jobManager.RemoveJob(jobID); err != nil {
		return NewErrorResponse(err)
	}

	resp := NewSuccessResponse()
	resp.Data["job_id"] = jobID
	resp.Data["pid"] = pid
	return resp
}

// handleStopAll handles a stop_all request
func (d *Daemon) handleStopAll(req *Request) *Response {
	stopped := d.jobManager.StopAll()

	resp := NewSuccessResponse()
	resp.Data["stopped"] = stopped
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

	// Check if job exists first
	_, err := d.jobManager.GetJob(jobID)
	if err != nil {
		return NewErrorResponse(err)
	}

	run := d.jobManager.GetCurrentRun(jobID)
	if run == nil {
		return NewErrorResponse(fmt.Errorf("job %s is not running", jobID))
	}

	if err := d.jobManager.Signal(jobID, syscall.Signal(int(signalNum))); err != nil {
		return NewErrorResponse(err)
	}

	resp := NewSuccessResponse()
	resp.Data["job_id"] = jobID
	resp.Data["pid"] = run.PID
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
	resp.Data["job"] = d.jobManager.jobToResponse(job)
	return resp
}

// handleVersion handles a version request
func (d *Daemon) handleVersion(req *Request) *Response {
	resp := NewSuccessResponse()
	resp.Data["version"] = version.Version
	resp.Data["running_jobs"] = d.countRunningJobs()
	return resp
}

// countRunningJobs returns the count of currently running jobs
func (d *Daemon) countRunningJobs() int {
	jobs := d.jobManager.ListJobs("")
	count := 0
	for _, job := range jobs {
		if job.IsRunning() {
			count++
		}
	}
	return count
}

// handleRuns handles a runs request
func (d *Daemon) handleRuns(req *Request) *Response {
	jobID, ok := req.Payload["job_id"].(string)
	if !ok {
		return NewErrorResponse(fmt.Errorf("missing job_id"))
	}

	runs, err := d.jobManager.ListRunsForJob(jobID)
	if err != nil {
		return NewErrorResponse(err)
	}

	var runResponses []RunResponse
	for _, run := range runs {
		runResponses = append(runResponses, runToResponse(run))
	}

	resp := NewSuccessResponse()
	resp.Data["runs"] = runResponses
	return resp
}

// handleStats handles a stats request
func (d *Daemon) handleStats(req *Request) *Response {
	jobID, ok := req.Payload["job_id"].(string)
	if !ok {
		return NewErrorResponse(fmt.Errorf("missing job_id"))
	}

	job, err := d.jobManager.GetJob(jobID)
	if err != nil {
		return NewErrorResponse(err)
	}

	resp := NewSuccessResponse()
	resp.Data["stats"] = jobToStats(job)
	return resp
}

// handlePorts handles a ports request
func (d *Daemon) handlePorts(req *Request) *Response {
	jobID, _ := req.Payload["job_id"].(string)
	workdir, _ := req.Payload["workdir"].(string)

	resp := NewSuccessResponse()

	if jobID != "" {
		// Refresh and get ports for specific job (triggers event if changed)
		jobPorts, err := d.jobManager.RefreshJobPorts(jobID)
		if err != nil {
			return NewErrorResponse(err)
		}
		resp.Data["ports"] = jobPorts
	} else {
		// Refresh and get ports for all running jobs (triggers events if changed)
		allPorts, err := d.jobManager.RefreshAllJobPorts(workdir)
		if err != nil {
			return NewErrorResponse(err)
		}
		resp.Data["ports"] = allPorts
	}

	return resp
}

// sendErrorResponse sends an error response to the client
func (d *Daemon) sendErrorResponse(encoder *json.Encoder, err error) {
	resp := NewErrorResponse(err)
	encoder.Encode(resp)
}

// handleSubscribe handles a subscribe request
func (d *Daemon) handleSubscribe(req *Request, conn net.Conn, encoder *json.Encoder) {
	workdir, _ := req.Payload["workdir"].(string)

	// Create subscriber
	sub := &Subscriber{
		conn:    conn,
		encoder: encoder,
		workdir: workdir,
	}

	// Add to subscribers list
	d.subscribersMu.Lock()
	d.subscribers = append(d.subscribers, sub)
	d.subscribersMu.Unlock()

	Logger.Debug("subscriber added", "workdir", workdir, "total", len(d.subscribers))

	// Send success response
	resp := NewSuccessResponse()
	resp.Data["message"] = "subscribed"
	if err := encoder.Encode(resp); err != nil {
		Logger.Error("error sending subscribe response", "error", err)
		d.removeSubscriber(sub)
		conn.Close()
		return
	}

	// Keep connection open and wait for it to close
	// The connection will be closed when the client disconnects or daemon shuts down
	// We detect this by trying to read (which will block until close or error)
	buf := make([]byte, 1)
	for {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		_, err := conn.Read(buf)
		if err != nil {
			// Connection closed or error
			break
		}
	}

	// Remove subscriber
	d.removeSubscriber(sub)
	conn.Close()
	Logger.Debug("subscriber removed", "total", len(d.subscribers))
}

// broadcastEvent sends an event to all subscribed clients
func (d *Daemon) broadcastEvent(event Event) {
	d.subscribersMu.RLock()
	subscribers := make([]*Subscriber, len(d.subscribers))
	copy(subscribers, d.subscribers)
	d.subscribersMu.RUnlock()

	var deadSubscribers []*Subscriber

	for _, sub := range subscribers {
		// Check workdir filter
		if sub.workdir != "" && event.Job.Workdir != sub.workdir {
			continue
		}

		// Set write deadline to avoid blocking
		sub.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := sub.encoder.Encode(event); err != nil {
			Logger.Error("error sending event to subscriber", "error", err)
			deadSubscribers = append(deadSubscribers, sub)
		}
	}

	// Remove dead subscribers
	for _, sub := range deadSubscribers {
		d.removeSubscriber(sub)
		sub.conn.Close()
	}
}

// removeSubscriber removes a subscriber from the list
func (d *Daemon) removeSubscriber(sub *Subscriber) {
	d.subscribersMu.Lock()
	defer d.subscribersMu.Unlock()

	for i, s := range d.subscribers {
		if s == sub {
			d.subscribers = append(d.subscribers[:i], d.subscribers[i+1:]...)
			return
		}
	}
}

// handleEvent processes events from the job manager
func (d *Daemon) handleEvent(event Event) {
	// Broadcast to subscribers
	d.broadcastEvent(event)
}

// recoverFromCrash handles cleanup after a daemon crash
func (d *Daemon) recoverFromCrash() error {
	// Find all runs marked as 'running' (they're orphans now)
	orphans, err := d.store.FindOrphanRuns()
	if err != nil {
		return fmt.Errorf("failed to find orphan runs: %w", err)
	}

	for _, orphan := range orphans {
		run := orphan.Run
		Logger.Info("found orphan run", "id", run.ID, "pid", run.PID)

		// Verify this is actually our process (PIDs can be reused!)
		if isOurProcess(run.PID, run.StartedAt, orphan.Command) {
			Logger.Info("killing orphan process", "pid", run.PID)

			// Signal the process group
			syscall.Kill(-run.PID, syscall.SIGTERM)

			// Wait briefly for graceful shutdown
			time.Sleep(2 * time.Second)

			// Force kill if still running
			if processExists(run.PID) {
				syscall.Kill(-run.PID, syscall.SIGKILL)
			}
		} else {
			Logger.Info("orphan process no longer exists or doesn't match", "pid", run.PID)
		}

		// Mark run as stopped
		if err := d.store.MarkRunStopped(run.ID); err != nil {
			Logger.Warn("failed to mark run as stopped", "id", run.ID, "error", err)
		}
	}

	return nil
}
