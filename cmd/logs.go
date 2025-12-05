package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/juanibiapina/gob/internal/tail"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Follow both stdout and stderr for jobs",
	Long: `Follow both stdout and stderr output for background jobs in real-time.

Follows all jobs started in the current directory.

OUTPUT FORMAT:
  Each line is prefixed with a tag in square brackets:
  - [<job_id>] default color prefix for stdout lines
  - [<job_id>] yellow prefix for stderr lines
  - [gob] cyan prefix for system events (process started/stopped)

  For raw output without prefixes, use the stdout and stderr commands instead.

Example:
  # Follow all jobs in current directory
  gob logs

Output:
  [gob] process started: ./my-server (pid:12345 id:abc)
  [V3x0QqI] Server listening on port 8080
  [V3x0QqI] Error: connection refused (yellow prefix)
  [gob] process stopped: ./my-server (pid:12345 id:abc)

Notes:
  - Streams output in real-time as it's written
  - Automatically picks up new jobs that start in the directory
  - Press Ctrl+C to stop following

Exit codes:
  0: Stopped by user (Ctrl+C)
  1: Error (log files not available)`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get current workdir
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Connect to daemon for initial list
		listClient, err := daemon.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		defer listClient.Close()

		if err := listClient.Connect(); err != nil {
			return fmt.Errorf("failed to connect to daemon: %w", err)
		}

		follower := tail.NewFollower(os.Stdout)
		var mu sync.Mutex
		type runningJob struct {
			pid     int
			command string
		}
		runningJobs := make(map[string]runningJob) // jobID -> job info

		// addJobSources adds stdout and stderr sources for a job (must be called with mu held)
		addJobSources := func(job *daemon.JobResponse) error {
			stdoutPath := job.StdoutPath
			stderrPath := job.StderrPath

			// Check if log files exist
			if _, err := os.Stat(stdoutPath); os.IsNotExist(err) {
				return fmt.Errorf("stdout log file not found: %s", stdoutPath)
			}
			if _, err := os.Stat(stderrPath); os.IsNotExist(err) {
				return fmt.Errorf("stderr log file not found: %s", stderrPath)
			}

			// Yellow ANSI color for stderr prefix (uses terminal theme)
			stderrPrefix := fmt.Sprintf("\033[33m[%s]\033[0m ", job.ID)
			stdoutPrefix := fmt.Sprintf("[%s] ", job.ID)

			follower.AddSource(tail.FileSource{Path: stdoutPath, Prefix: stdoutPrefix})
			follower.AddSource(tail.FileSource{Path: stderrPath, Prefix: stderrPrefix})

			runningJobs[job.ID] = runningJob{pid: job.PID, command: strings.Join(job.Command, " ")}
			return nil
		}

		// Get initial jobs
		jobs, err := listClient.List(cwd)
		if err != nil {
			return fmt.Errorf("failed to list jobs: %w", err)
		}

		// Add initial jobs (no "process started" log since they were already running)
		mu.Lock()
		for _, job := range jobs {
			if err := addJobSources(&job); err != nil {
				mu.Unlock()
				return err
			}
		}
		hasInitialJobs := len(jobs) > 0
		mu.Unlock()

		// Connect a separate client for event subscription
		eventClient, err := daemon.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create event client: %w", err)
		}
		defer eventClient.Close()

		if err := eventClient.Connect(); err != nil {
			return fmt.Errorf("failed to connect event client: %w", err)
		}

		// Subscribe to events for this workdir
		eventCh, errCh := eventClient.SubscribeChan(cwd)

		// Handle events in a goroutine
		go func() {
			for {
				select {
				case event, ok := <-eventCh:
					if !ok {
						return
					}
					mu.Lock()
					switch event.Type {
					case daemon.EventTypeJobAdded, daemon.EventTypeJobStarted:
						if event.Job.Status == "running" {
							// Only add if not already tracking
							if _, exists := runningJobs[event.JobID]; !exists {
								addJobSources(&event.Job)
								follower.SystemLog("process started: %s (pid:%d id:%s)",
									strings.Join(event.Job.Command, " "), event.Job.PID, event.JobID)
							}
						}
					case daemon.EventTypeJobStopped:
						if job, exists := runningJobs[event.JobID]; exists {
							if event.Job.ExitCode != nil {
								follower.SystemLog("process stopped: %s (pid:%d id:%s) exit: %d",
									job.command, job.pid, event.JobID, *event.Job.ExitCode)
							} else {
								follower.SystemLog("process stopped: %s (pid:%d id:%s)",
									job.command, job.pid, event.JobID)
							}
							delete(runningJobs, event.JobID)
							// Check if all jobs have stopped
							if len(runningJobs) == 0 {
								follower.SystemLog("all processes stopped")
							}
						}
					}
					mu.Unlock()
				case err, ok := <-errCh:
					if ok && err != nil {
						// Log error but don't exit - logs can continue without events
						follower.SystemLog("event subscription error: %v", err)
					}
					return
				}
			}
		}()

		// If no initial jobs, log and wait (events will add them)
		if !hasInitialJobs {
			follower.SystemLog("waiting for jobs...")
		}

		return follower.Wait()
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
}
