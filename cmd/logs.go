package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/juanibiapina/gob/internal/process"
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
  - [<job_id>] white prefix for stdout lines
  - [<job_id>] orange prefix for stderr lines
  - [monitor] cyan prefix for system events (process started/stopped)

  For raw output without prefixes, use the stdout and stderr commands instead.

Example:
  # Follow all jobs in current directory
  gob logs

Output:
  [monitor] process started: ./my-server (pid:12345 id:V3x0QqI)
  [V3x0QqI] Server listening on port 8080
  [V3x0QqI] Error: connection refused (orange prefix)
  [monitor] process stopped: ./my-server (pid:12345 id:V3x0QqI)

Notes:
  - Streams output in real-time as it's written
  - Automatically picks up new jobs that start in the directory
  - Press Ctrl+C to stop following

Exit codes:
  0: Stopped by user (Ctrl+C)
  1: Error (log files not available)`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Connect to daemon
		client, err := daemon.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		defer client.Close()

		if err := client.Connect(); err != nil {
			return fmt.Errorf("failed to connect to daemon: %w", err)
		}

		// Get current workdir
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		follower := tail.NewFollower(os.Stdout)
		var mu sync.Mutex
		knownJobs := make(map[string]bool)
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

			// Orange ANSI color for stderr prefix
			orangePrefix := fmt.Sprintf("\033[38;5;208m[%s]\033[0m ", job.ID)
			stdoutPrefix := fmt.Sprintf("[%s] ", job.ID)

			follower.AddSource(tail.FileSource{Path: stdoutPath, Prefix: stdoutPrefix})
			follower.AddSource(tail.FileSource{Path: stderrPath, Prefix: orangePrefix})

			runningJobs[job.ID] = runningJob{pid: job.PID, command: strings.Join(job.Command, " ")}
			return nil
		}

		// Get initial jobs
		jobs, err := client.List(cwd)
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
			knownJobs[job.ID] = true
		}
		mu.Unlock()

		// Start goroutine to watch for new jobs and detect stopped jobs
		go func() {
			for {
				time.Sleep(500 * time.Millisecond)

				mu.Lock()
				// Check for stopped jobs - collect IDs to delete first
				var stoppedJobs []string
				for jobID, job := range runningJobs {
					if !process.IsProcessRunning(job.pid) {
						stoppedJobs = append(stoppedJobs, jobID)
					}
				}
				// Now delete and log
				for _, jobID := range stoppedJobs {
					job := runningJobs[jobID]
					follower.SystemLog("process stopped: %s (pid:%d id:%s)", job.command, job.pid, jobID)
					delete(runningJobs, jobID)
				}
				// Check if all jobs have stopped
				if len(stoppedJobs) > 0 && len(runningJobs) == 0 {
					follower.SystemLog("all processes stopped")
				}
				mu.Unlock()

				// Check for new jobs - need to reconnect for each request
				newClient, err := daemon.NewClient()
				if err != nil {
					continue
				}
				if err := newClient.Connect(); err != nil {
					newClient.Close()
					continue
				}
				newJobs, err := newClient.List(cwd)
				newClient.Close()
				if err != nil {
					continue
				}

				mu.Lock()
				for _, job := range newJobs {
					if !knownJobs[job.ID] {
						knownJobs[job.ID] = true
						addJobSources(&job)
						follower.SystemLog("process started: %s (pid:%d id:%s)", strings.Join(job.Command, " "), job.PID, job.ID)
					}
				}
				mu.Unlock()
			}
		}()

		// If no initial jobs, wait for first job to appear
		if len(jobs) == 0 {
			follower.SystemLog("waiting for jobs...")
			for {
				time.Sleep(500 * time.Millisecond)

				// Reconnect for each request
				newClient, err := daemon.NewClient()
				if err != nil {
					continue
				}
				if err := newClient.Connect(); err != nil {
					newClient.Close()
					continue
				}
				newJobs, err := newClient.List(cwd)
				newClient.Close()
				if err != nil {
					continue
				}

				if len(newJobs) > 0 {
					mu.Lock()
					for _, job := range newJobs {
						if !knownJobs[job.ID] {
							knownJobs[job.ID] = true
							if err := addJobSources(&job); err != nil {
								mu.Unlock()
								return err
							}
							follower.SystemLog("process started: %s (pid:%d id:%s)", strings.Join(job.Command, " "), job.PID, job.ID)
						}
					}
					mu.Unlock()
					break
				}
			}
		}

		return follower.Wait()
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
}
