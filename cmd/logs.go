package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/juanibiapina/gob/internal/process"
	"github.com/juanibiapina/gob/internal/storage"
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
		jobDir, err := storage.GetJobDir()
		if err != nil {
			return fmt.Errorf("failed to get job directory: %w", err)
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
		addJobSources := func(jobID string, pid int, command string) error {
			stdoutPath := filepath.Join(jobDir, fmt.Sprintf("%s.stdout.log", jobID))
			stderrPath := filepath.Join(jobDir, fmt.Sprintf("%s.stderr.log", jobID))

			// Check if log files exist
			if _, err := os.Stat(stdoutPath); os.IsNotExist(err) {
				return fmt.Errorf("stdout log file not found: %s", stdoutPath)
			}
			if _, err := os.Stat(stderrPath); os.IsNotExist(err) {
				return fmt.Errorf("stderr log file not found: %s", stderrPath)
			}

			// Orange ANSI color for stderr prefix
			orangePrefix := fmt.Sprintf("\033[38;5;208m[%s]\033[0m ", jobID)
			stdoutPrefix := fmt.Sprintf("[%s] ", jobID)

			follower.AddSource(tail.FileSource{Path: stdoutPath, Prefix: stdoutPrefix})
			follower.AddSource(tail.FileSource{Path: stderrPath, Prefix: orangePrefix})

			runningJobs[jobID] = runningJob{pid: pid, command: command}
			return nil
		}

		// formatCommand returns a short representation of the command
		formatCommand := func(command []string) string {
			if len(command) == 0 {
				return ""
			}
			return strings.Join(command, " ")
		}

		// Follow all jobs in current directory and watch for new ones
		jobs, err := storage.ListJobMetadata()
			if err != nil {
				return fmt.Errorf("failed to list jobs: %w", err)
			}

			// Add initial jobs (no "process started" log since they were already running)
			mu.Lock()
			for _, job := range jobs {
				if err := addJobSources(job.ID, job.Metadata.PID, formatCommand(job.Metadata.Command)); err != nil {
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

					// Check for new jobs
					jobs, err := storage.ListJobMetadata()
					if err != nil {
						continue
					}
					mu.Lock()
					for _, job := range jobs {
						if !knownJobs[job.ID] {
							knownJobs[job.ID] = true
							addJobSources(job.ID, job.Metadata.PID, formatCommand(job.Metadata.Command))
							follower.SystemLog("process started: %s (pid:%d id:%s)", formatCommand(job.Metadata.Command), job.Metadata.PID, job.ID)
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
				jobs, err := storage.ListJobMetadata()
				if err != nil {
					continue
				}
				if len(jobs) > 0 {
					mu.Lock()
					for _, job := range jobs {
						if !knownJobs[job.ID] {
							knownJobs[job.ID] = true
							if err := addJobSources(job.ID, job.Metadata.PID, formatCommand(job.Metadata.Command)); err != nil {
								mu.Unlock()
								return err
							}
							follower.SystemLog("process started: %s (pid:%d id:%s)", formatCommand(job.Metadata.Command), job.Metadata.PID, job.ID)
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
