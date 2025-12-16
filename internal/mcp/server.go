// Package mcp provides an MCP (Model Context Protocol) server for gob.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/juanibiapina/gob/internal/telemetry"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the MCP server with gob-specific functionality.
type Server struct {
	mcpServer *server.MCPServer
}

// NewServer creates a new MCP server for gob.
func NewServer(version string) *Server {
	s := &Server{}

	// Create the MCP server
	s.mcpServer = server.NewMCPServer(
		"gob",
		version,
		server.WithToolCapabilities(false),
		server.WithRecovery(),
	)

	// Register tools
	s.registerTools()

	return s
}

// Serve starts the MCP server on stdio.
func (s *Server) Serve() error {
	return server.ServeStdio(s.mcpServer)
}

// ListToolNames returns the names of all registered MCP tools.
func (s *Server) ListToolNames() []string {
	var names []string
	for name := range s.mcpServer.ListTools() {
		names = append(names, name)
	}
	return names
}

// registerTools registers all MCP tools.
func (s *Server) registerTools() {
	s.registerJobAdd()
	s.registerJobList()
	s.registerJobStop()
	s.registerJobStart()
	s.registerJobRemove()
	s.registerJobAwait()
	s.registerJobAwaitAny()
	s.registerJobAwaitAll()
	s.registerJobRestart()
	s.registerJobRun()
	s.registerJobSignal()
	s.registerJobStdout()
	s.registerJobStderr()
	s.registerJobRuns()
	s.registerJobStats()
	s.registerJobPorts()
}

// connectToDaemon creates and connects a daemon client.
func connectToDaemon() (*daemon.Client, error) {
	client, err := daemon.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	if err := client.Connect(); err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	return client, nil
}

// jsonResult marshals a result to JSON and returns a tool result.
func jsonResult(result any) (*mcp.CallToolResult, error) {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
	}
	return mcp.NewToolResultText(string(resultJSON)), nil
}

// toolHandler is the type for MCP tool handlers.
type toolHandler func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)

// addTool registers a tool with telemetry tracking.
func (s *Server) addTool(tool mcp.Tool, handler toolHandler) {
	s.mcpServer.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		telemetry.MCPToolCall(tool.Name)
		return handler(ctx, request)
	})
}

// registerJobAdd registers the gob_add tool.
func (s *Server) registerJobAdd() {
	tool := mcp.NewTool("gob_add",
		mcp.WithDescription("Create a new background job in the current directory"),
		mcp.WithArray("command",
			mcp.Required(),
			mcp.Description("Command and arguments as array (e.g. [\"make\", \"build\"])"),
		),
	)

	s.addTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		command, err := request.RequireStringSlice("command")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if len(command) == 0 {
			return mcp.NewToolResultError("command array cannot be empty"), nil
		}

		workdir, err := os.Getwd()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get current directory: %v", err)), nil
		}

		// Capture current environment
		env := os.Environ()

		client, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer client.Close()

		result, err := client.Add(command, workdir, env)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to add job: %v", err)), nil
		}

		response := map[string]any{
			"job_id": result.Job.ID,
			"status": result.Job.Status,
			"pid":    result.Job.PID,
		}

		// Include stats if job has previous runs
		if result.Stats != nil && result.Stats.RunCount > 0 {
			response["previous_runs"] = result.Stats.RunCount
			response["success_rate"] = result.Stats.SuccessRate
			response["expected_duration_ms"] = result.Stats.AvgDurationMs
		}

		return jsonResult(response)
	})
}

// registerJobList registers the gob_list tool.
func (s *Server) registerJobList() {
	tool := mcp.NewTool("gob_list",
		mcp.WithDescription("List jobs in current directory"),
		mcp.WithBoolean("all",
			mcp.Description("Include jobs from all directories (default: false)"),
		),
	)

	s.addTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		all := request.GetBool("all", false)

		var workdir string
		if !all {
			var err error
			workdir, err = os.Getwd()
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get current directory: %v", err)), nil
			}
		}

		client, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer client.Close()

		jobs, err := client.List(workdir)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list jobs: %v", err)), nil
		}

		// Build response
		jobList := make([]map[string]any, 0, len(jobs))
		for _, job := range jobs {
			jobInfo := map[string]any{
				"job_id":  job.ID,
				"status":  job.Status,
				"command": job.Command,
				"workdir": job.Workdir,
				"pid":     job.PID,
			}
			if job.ExitCode != nil {
				jobInfo["exit_code"] = *job.ExitCode
			}
			jobList = append(jobList, jobInfo)
		}

		return jsonResult(map[string]any{"jobs": jobList})
	})
}

// registerJobStop registers the gob_stop tool.
func (s *Server) registerJobStop() {
	tool := mcp.NewTool("gob_stop",
		mcp.WithDescription("Stop a running job"),
		mcp.WithString("job_id",
			mcp.Required(),
			mcp.Description("Job ID (3-character identifier)"),
		),
		mcp.WithBoolean("force",
			mcp.Description("Use SIGKILL instead of SIGTERM (default: false)"),
		),
	)

	s.addTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobID, err := request.RequireString("job_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		force := request.GetBool("force", false)

		client, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer client.Close()

		_, err = client.Stop(jobID, force)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to stop job: %v", err)), nil
		}

		// Get the job to return its status
		job, err := client.GetJob(jobID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get job: %v", err)), nil
		}

		result := map[string]any{
			"job_id": job.ID,
			"status": job.Status,
		}
		if job.ExitCode != nil {
			result["exit_code"] = *job.ExitCode
		}

		return jsonResult(result)
	})
}

// registerJobStart registers the gob_start tool.
func (s *Server) registerJobStart() {
	tool := mcp.NewTool("gob_start",
		mcp.WithDescription("Start a stopped job"),
		mcp.WithString("job_id",
			mcp.Required(),
			mcp.Description("Job ID"),
		),
	)

	s.addTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobID, err := request.RequireString("job_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		client, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer client.Close()

		// Capture current environment
		env := os.Environ()

		job, err := client.Start(jobID, env)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to start job: %v", err)), nil
		}

		return jsonResult(map[string]any{
			"job_id": job.ID,
			"status": job.Status,
			"pid":    job.PID,
		})
	})
}

// registerJobRemove registers the gob_remove tool.
func (s *Server) registerJobRemove() {
	tool := mcp.NewTool("gob_remove",
		mcp.WithDescription("Remove a stopped job"),
		mcp.WithString("job_id",
			mcp.Required(),
			mcp.Description("Job ID"),
		),
	)

	s.addTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobID, err := request.RequireString("job_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		client, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer client.Close()

		_, err = client.Remove(jobID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to remove job: %v", err)), nil
		}

		return jsonResult(map[string]any{
			"success": true,
			"job_id":  jobID,
		})
	})
}

// maxOutputSize is the maximum size of stdout/stderr to return (100KB)
const maxOutputSize = 100 * 1024

// readJobOutput reads the stdout and stderr of a job, truncating if necessary
func readJobOutput(job *daemon.JobResponse) (stdout, stderr string) {
	// Read stdout
	if job.StdoutPath != "" {
		if content, err := os.ReadFile(job.StdoutPath); err == nil {
			stdout = string(content)
			if len(stdout) > maxOutputSize {
				stdout = stdout[:maxOutputSize] + "\n... (truncated)"
			}
		}
	}

	// Read stderr
	stderrPath := strings.Replace(job.StdoutPath, ".stdout.log", ".stderr.log", 1)
	if content, err := os.ReadFile(stderrPath); err == nil {
		stderr = string(content)
		if len(stderr) > maxOutputSize {
			stderr = stderr[:maxOutputSize] + "\n... (truncated)"
		}
	}

	return stdout, stderr
}

// registerJobAwait registers the gob_await tool.
func (s *Server) registerJobAwait() {
	tool := mcp.NewTool("gob_await",
		mcp.WithDescription("Wait for a job to complete and return its output"),
		mcp.WithString("job_id",
			mcp.Required(),
			mcp.Description("Job ID"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Timeout in seconds (0 = no timeout, default: 300)"),
		),
	)

	s.addTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobID, err := request.RequireString("job_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		timeout := request.GetInt("timeout", 300)
		if timeout == 0 {
			timeout = 3600 // Max 1 hour
		}

		client, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer client.Close()

		// Get initial job state
		job, err := client.GetJob(jobID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get job: %v", err)), nil
		}

		// If already stopped, return immediately
		if job.Status == "stopped" {
			stdout, stderr := readJobOutput(job)
			result := map[string]any{
				"job_id": job.ID,
				"status": job.Status,
				"stdout": stdout,
				"stderr": stderr,
			}
			if job.ExitCode != nil {
				result["exit_code"] = *job.ExitCode
			}
			return jsonResult(result)
		}

		// Subscribe and wait for completion
		subClient, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer subClient.Close()

		eventCh, errCh := subClient.SubscribeChan("")
		timeoutCh := time.After(time.Duration(timeout) * time.Second)

		for {
			select {
			case <-timeoutCh:
				return mcp.NewToolResultError("timeout waiting for job to complete"), nil

			case err := <-errCh:
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("subscription error: %v", err)), nil
				}
				return mcp.NewToolResultError("subscription closed unexpectedly"), nil

			case event := <-eventCh:
				if event.Type == daemon.EventTypeJobStopped && event.JobID == jobID {
					stdout, stderr := readJobOutput(&event.Job)
					result := map[string]any{
						"job_id": event.Job.ID,
						"status": event.Job.Status,
						"stdout": stdout,
						"stderr": stderr,
					}
					if event.Job.ExitCode != nil {
						result["exit_code"] = *event.Job.ExitCode
					}
					return jsonResult(result)
				}
			}
		}
	})
}

// registerJobAwaitAny registers the gob_await_any tool.
func (s *Server) registerJobAwaitAny() {
	tool := mcp.NewTool("gob_await_any",
		mcp.WithDescription("Wait for any job in current directory to complete"),
		mcp.WithNumber("timeout",
			mcp.Description("Timeout in seconds (0 = no timeout, default: 300)"),
		),
	)

	s.addTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		timeout := request.GetInt("timeout", 300)
		if timeout == 0 {
			timeout = 3600
		}

		workdir, err := os.Getwd()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get current directory: %v", err)), nil
		}

		client, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer client.Close()

		// Get list of running jobs
		jobs, err := client.List(workdir)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list jobs: %v", err)), nil
		}

		// Build set of running job IDs
		watchingIDs := make(map[string]bool)
		for _, job := range jobs {
			if job.Status == "running" {
				watchingIDs[job.ID] = true
			}
		}

		if len(watchingIDs) == 0 {
			return jsonResult(map[string]any{
				"message": "no running jobs to await",
			})
		}

		// Subscribe and wait for any to complete
		subClient, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer subClient.Close()

		eventCh, errCh := subClient.SubscribeChan(workdir)
		timeoutCh := time.After(time.Duration(timeout) * time.Second)

		for {
			select {
			case <-timeoutCh:
				return mcp.NewToolResultError("timeout waiting for job to complete"), nil

			case err := <-errCh:
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("subscription error: %v", err)), nil
				}
				return mcp.NewToolResultError("subscription closed unexpectedly"), nil

			case event := <-eventCh:
				if event.Type == daemon.EventTypeJobStopped && watchingIDs[event.JobID] {
					stdout, stderr := readJobOutput(&event.Job)
					result := map[string]any{
						"job_id": event.Job.ID,
						"status": event.Job.Status,
						"stdout": stdout,
						"stderr": stderr,
					}
					if event.Job.ExitCode != nil {
						result["exit_code"] = *event.Job.ExitCode
					}
					return jsonResult(result)
				}
			}
		}
	})
}

// registerJobAwaitAll registers the gob_await_all tool.
func (s *Server) registerJobAwaitAll() {
	tool := mcp.NewTool("gob_await_all",
		mcp.WithDescription("Wait for all jobs in current directory to complete"),
		mcp.WithNumber("timeout",
			mcp.Description("Timeout in seconds (0 = no timeout, default: 300)"),
		),
	)

	s.addTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		timeout := request.GetInt("timeout", 300)
		if timeout == 0 {
			timeout = 3600
		}

		workdir, err := os.Getwd()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get current directory: %v", err)), nil
		}

		client, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer client.Close()

		// Get list of running jobs
		jobs, err := client.List(workdir)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list jobs: %v", err)), nil
		}

		// Build set of running job IDs
		pendingIDs := make(map[string]bool)
		for _, job := range jobs {
			if job.Status == "running" {
				pendingIDs[job.ID] = true
			}
		}

		if len(pendingIDs) == 0 {
			return jsonResult(map[string]any{
				"jobs":          []map[string]any{},
				"all_succeeded": true,
			})
		}

		// Subscribe and wait for all to complete
		subClient, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer subClient.Close()

		eventCh, errCh := subClient.SubscribeChan(workdir)
		timeoutCh := time.After(time.Duration(timeout) * time.Second)

		completedJobs := make([]map[string]any, 0)
		allSucceeded := true

		for len(pendingIDs) > 0 {
			select {
			case <-timeoutCh:
				return mcp.NewToolResultError("timeout waiting for jobs to complete"), nil

			case err := <-errCh:
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("subscription error: %v", err)), nil
				}
				return mcp.NewToolResultError("subscription closed unexpectedly"), nil

			case event := <-eventCh:
				if event.Type == daemon.EventTypeJobStopped && pendingIDs[event.JobID] {
					delete(pendingIDs, event.JobID)

					jobResult := map[string]any{
						"job_id": event.Job.ID,
						"status": event.Job.Status,
					}
					if event.Job.ExitCode != nil {
						jobResult["exit_code"] = *event.Job.ExitCode
						if *event.Job.ExitCode != 0 {
							allSucceeded = false
						}
					} else {
						allSucceeded = false
					}
					completedJobs = append(completedJobs, jobResult)
				}
			}
		}

		return jsonResult(map[string]any{
			"jobs":          completedJobs,
			"all_succeeded": allSucceeded,
		})
	})
}

// registerJobRestart registers the gob_restart tool.
func (s *Server) registerJobRestart() {
	tool := mcp.NewTool("gob_restart",
		mcp.WithDescription("Stop and start a job"),
		mcp.WithString("job_id",
			mcp.Required(),
			mcp.Description("Job ID"),
		),
	)

	s.addTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobID, err := request.RequireString("job_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		client, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer client.Close()

		// Capture current environment
		env := os.Environ()

		job, err := client.Restart(jobID, env)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to restart job: %v", err)), nil
		}

		return jsonResult(map[string]any{
			"job_id": job.ID,
			"status": job.Status,
			"pid":    job.PID,
		})
	})
}

// registerJobRun registers the gob_run tool.
func (s *Server) registerJobRun() {
	tool := mcp.NewTool("gob_run",
		mcp.WithDescription("Add a job and wait for it to complete, returning its output"),
		mcp.WithArray("command",
			mcp.Required(),
			mcp.Description("Command and arguments as array (e.g. [\"make\", \"build\"])"),
		),
		mcp.WithNumber("timeout",
			mcp.Description("Timeout in seconds (0 = no timeout, default: 300)"),
		),
	)

	s.addTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		command, err := request.RequireStringSlice("command")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if len(command) == 0 {
			return mcp.NewToolResultError("command array cannot be empty"), nil
		}

		timeout := request.GetInt("timeout", 300)
		if timeout == 0 {
			timeout = 3600 // Max 1 hour
		}

		workdir, err := os.Getwd()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get current directory: %v", err)), nil
		}

		// Capture current environment
		env := os.Environ()

		// Set up subscription BEFORE adding the job to avoid race conditions
		// with very fast commands that complete before subscription is ready
		subClient, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer subClient.Close()

		eventCh, errCh := subClient.SubscribeChan("")

		client, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer client.Close()

		// Add the job
		addResult, err := client.Add(command, workdir, env)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to add job: %v", err)), nil
		}

		jobID := addResult.Job.ID

		// Build initial response with stats if available
		response := map[string]any{
			"job_id": jobID,
		}

		// Include stats if job has previous runs
		if addResult.Stats != nil && addResult.Stats.RunCount > 0 {
			response["previous_runs"] = addResult.Stats.RunCount
			response["success_rate"] = addResult.Stats.SuccessRate
			response["expected_duration_ms"] = addResult.Stats.AvgDurationMs
		}

		// If already stopped (very fast command), return immediately
		if addResult.Job.Status == "stopped" {
			stdout, stderr := readJobOutput(&daemon.JobResponse{
				ID:         addResult.Job.ID,
				StdoutPath: addResult.Job.StdoutPath,
			})
			response["status"] = addResult.Job.Status
			response["stdout"] = stdout
			response["stderr"] = stderr
			if addResult.Job.ExitCode != nil {
				response["exit_code"] = *addResult.Job.ExitCode
			}
			return jsonResult(response)
		}

		timeoutCh := time.After(time.Duration(timeout) * time.Second)

		for {
			select {
			case <-timeoutCh:
				return mcp.NewToolResultError("timeout waiting for job to complete"), nil

			case err := <-errCh:
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("subscription error: %v", err)), nil
				}
				return mcp.NewToolResultError("subscription closed unexpectedly"), nil

			case event := <-eventCh:
				if event.Type == daemon.EventTypeJobStopped && event.JobID == jobID {
					stdout, stderr := readJobOutput(&event.Job)
					response["status"] = event.Job.Status
					response["stdout"] = stdout
					response["stderr"] = stderr
					if event.Job.ExitCode != nil {
						response["exit_code"] = *event.Job.ExitCode
					}
					return jsonResult(response)
				}
			}
		}
	})
}

// signalMap maps signal names to syscall.Signal values
var signalMap = map[string]syscall.Signal{
	"HUP":  syscall.SIGHUP,
	"INT":  syscall.SIGINT,
	"QUIT": syscall.SIGQUIT,
	"KILL": syscall.SIGKILL,
	"TERM": syscall.SIGTERM,
	"USR1": syscall.SIGUSR1,
	"USR2": syscall.SIGUSR2,
	"STOP": syscall.SIGSTOP,
	"CONT": syscall.SIGCONT,
	"ALRM": syscall.SIGALRM,
	"PIPE": syscall.SIGPIPE,
	"CHLD": syscall.SIGCHLD,
	"ABRT": syscall.SIGABRT,
	"TRAP": syscall.SIGTRAP,
}

// parseSignal converts a signal name or number to a syscall.Signal
func parseSignal(signalStr string) (syscall.Signal, error) {
	// Try to parse as number first
	if num, err := strconv.Atoi(signalStr); err == nil {
		return syscall.Signal(num), nil
	}

	// Parse as signal name - remove "SIG" prefix if present
	upperStr := strings.ToUpper(signalStr)
	normalizedStr := strings.TrimPrefix(upperStr, "SIG")

	if sig, ok := signalMap[normalizedStr]; ok {
		return sig, nil
	}

	return 0, fmt.Errorf("invalid signal: %s", signalStr)
}

// registerJobSignal registers the gob_signal tool.
func (s *Server) registerJobSignal() {
	tool := mcp.NewTool("gob_signal",
		mcp.WithDescription("Send a signal to a running job"),
		mcp.WithString("job_id",
			mcp.Required(),
			mcp.Description("Job ID"),
		),
		mcp.WithString("signal",
			mcp.Required(),
			mcp.Description("Signal name (HUP, SIGTERM, etc.) or number (1, 15, etc.)"),
		),
	)

	s.addTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobID, err := request.RequireString("job_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		signalStr, err := request.RequireString("signal")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Convert signal name/number to syscall.Signal
		sig, err := parseSignal(signalStr)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		client, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer client.Close()

		pid, err := client.Signal(jobID, sig)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to send signal: %v", err)), nil
		}

		return jsonResult(map[string]any{
			"success": true,
			"job_id":  jobID,
			"signal":  signalStr,
			"pid":     pid,
		})
	})
}

// registerJobStdout registers the gob_stdout tool.
func (s *Server) registerJobStdout() {
	tool := mcp.NewTool("gob_stdout",
		mcp.WithDescription("Read stdout from a job"),
		mcp.WithString("job_id",
			mcp.Required(),
			mcp.Description("Job ID"),
		),
	)

	s.addTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobID, err := request.RequireString("job_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		client, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer client.Close()

		job, err := client.GetJob(jobID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get job: %v", err)), nil
		}

		var content string
		if job.StdoutPath != "" {
			if data, err := os.ReadFile(job.StdoutPath); err == nil {
				content = string(data)
				if len(content) > maxOutputSize {
					content = content[:maxOutputSize] + "\n... (truncated)"
				}
			}
		}

		return jsonResult(map[string]any{
			"job_id":  jobID,
			"content": content,
		})
	})
}

// registerJobStderr registers the gob_stderr tool.
func (s *Server) registerJobStderr() {
	tool := mcp.NewTool("gob_stderr",
		mcp.WithDescription("Read stderr from a job"),
		mcp.WithString("job_id",
			mcp.Required(),
			mcp.Description("Job ID"),
		),
	)

	s.addTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobID, err := request.RequireString("job_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		client, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer client.Close()

		job, err := client.GetJob(jobID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get job: %v", err)), nil
		}

		var content string
		stderrPath := strings.Replace(job.StdoutPath, ".stdout.log", ".stderr.log", 1)
		if data, err := os.ReadFile(stderrPath); err == nil {
			content = string(data)
			if len(content) > maxOutputSize {
				content = content[:maxOutputSize] + "\n... (truncated)"
			}
		}

		return jsonResult(map[string]any{
			"job_id":  jobID,
			"content": content,
		})
	})
}

// registerJobRuns registers the gob_runs tool.
func (s *Server) registerJobRuns() {
	tool := mcp.NewTool("gob_runs",
		mcp.WithDescription("Show run history for a job"),
		mcp.WithString("job_id",
			mcp.Required(),
			mcp.Description("Job ID"),
		),
	)

	s.addTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobID, err := request.RequireString("job_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		client, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer client.Close()

		runs, err := client.Runs(jobID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get runs: %v", err)), nil
		}

		// Build response
		runList := make([]map[string]any, 0, len(runs))
		for _, run := range runs {
			runInfo := map[string]any{
				"run_id":      run.ID,
				"job_id":      run.JobID,
				"status":      run.Status,
				"started_at":  run.StartedAt,
				"duration_ms": run.DurationMs,
				"pid":         run.PID,
			}
			if run.ExitCode != nil {
				runInfo["exit_code"] = *run.ExitCode
			}
			if run.StoppedAt != "" {
				runInfo["stopped_at"] = run.StoppedAt
			}
			runList = append(runList, runInfo)
		}

		return jsonResult(map[string]any{"runs": runList})
	})
}

// registerJobStats registers the gob_stats tool.
func (s *Server) registerJobStats() {
	tool := mcp.NewTool("gob_stats",
		mcp.WithDescription("Show statistics for a job"),
		mcp.WithString("job_id",
			mcp.Required(),
			mcp.Description("Job ID"),
		),
	)

	s.addTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobID, err := request.RequireString("job_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		client, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer client.Close()

		stats, err := client.Stats(jobID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get stats: %v", err)), nil
		}

		return jsonResult(map[string]any{
			"job_id":            stats.JobID,
			"command":           stats.Command,
			"run_count":         stats.RunCount,
			"success_count":     stats.SuccessCount,
			"success_rate":      stats.SuccessRate,
			"total_duration_ms": stats.TotalDurationMs,
			"avg_duration_ms":   stats.AvgDurationMs,
			"min_duration_ms":   stats.MinDurationMs,
			"max_duration_ms":   stats.MaxDurationMs,
		})
	})
}

// registerJobPorts registers the gob_ports tool.
func (s *Server) registerJobPorts() {
	tool := mcp.NewTool("gob_ports",
		mcp.WithDescription("List listening ports for a job's process tree"),
		mcp.WithString("job_id",
			mcp.Description("Job ID (optional, lists all running jobs if omitted)"),
		),
	)

	s.addTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		client, err := connectToDaemon()
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		defer client.Close()

		jobID := request.GetString("job_id", "")

		if jobID != "" {
			// Get ports for specific job
			ports, err := client.Ports(jobID)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to get ports: %v", err)), nil
			}

			// Return result (includes status/message for stopped jobs)
			return jsonResult(ports)
		}

		// Get ports for all running jobs in current directory
		workdir, err := os.Getwd()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get current directory: %v", err)), nil
		}

		allPorts, err := client.AllPorts(workdir)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get ports: %v", err)), nil
		}

		return jsonResult(map[string]any{"jobs": allPorts})
	})
}
