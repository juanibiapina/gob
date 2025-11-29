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
	Use:               "logs [job_id]",
	Short:             "Follow both stdout and stderr for jobs",
	ValidArgsFunction: completeJobIDs,
	Long: `Follow both stdout and stderr output for background jobs in real-time.

Without arguments, follows all jobs started in the current directory.
With a job ID, follows only that specific job.

Each line is prefixed with the job ID. Stderr lines have the prefix colored
orange to distinguish them from stdout.

Example:
  # Follow all jobs in current directory
  gob logs

  # Follow a specific job
  gob logs V3x0QqI

Output:
  [V3x0QqI] stdout line 1
  [V3x0QqI] error message (orange prefix)
  [V3x0QqI] stdout line 2

Notes:
  - Only works for jobs that have log files (jobs started with logging enabled)
  - Streams output in real-time as it's written
  - Automatically picks up new jobs that start in the directory
  - Press Ctrl+C to stop following

Exit codes:
  0: Stopped by user (Ctrl+C)
  1: Error (job not found, log files not available)`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobDir, err := storage.GetJobDir()
		if err != nil {
			return fmt.Errorf("failed to get job directory: %w", err)
		}

		follower := tail.NewFollower(os.Stdout)
		var mu sync.Mutex
		knownJobs := make(map[string]bool)
		runningJobs := make(map[string]int) // jobID -> PID

		// addJobSources adds stdout and stderr sources for a job (must be called with mu held)
		addJobSources := func(jobID string, pid int) error {
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

			runningJobs[jobID] = pid
			return nil
		}

		// formatCommand returns a short representation of the command
		formatCommand := func(command []string) string {
			if len(command) == 0 {
				return ""
			}
			return strings.Join(command, " ")
		}

		if len(args) == 0 {
			// No arguments: follow all jobs in current directory and watch for new ones
			jobs, err := storage.ListJobMetadata()
			if err != nil {
				return fmt.Errorf("failed to list jobs: %w", err)
			}

			// Add initial jobs (no "process started" log since they were already running)
			mu.Lock()
			for _, job := range jobs {
				if err := addJobSources(job.ID, job.Metadata.PID); err != nil {
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
					for jobID, pid := range runningJobs {
						if !process.IsProcessRunning(pid) {
							stoppedJobs = append(stoppedJobs, jobID)
						}
					}
					// Now delete and log
					for _, jobID := range stoppedJobs {
						follower.SystemLog("process stopped: %s", jobID)
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
							addJobSources(job.ID, job.Metadata.PID)
							follower.SystemLog("process started: %s (%s)", job.ID, formatCommand(job.Metadata.Command))
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
								if err := addJobSources(job.ID, job.Metadata.PID); err != nil {
									mu.Unlock()
									return err
								}
								follower.SystemLog("process started: %s (%s)", job.ID, formatCommand(job.Metadata.Command))
							}
						}
						mu.Unlock()
						break
					}
				}
			}
		} else {
			// Specific job ID provided - no dynamic watching
			jobID := args[0]
			metadata, err := storage.LoadJobMetadata(jobID + ".json")
			if err != nil {
				return fmt.Errorf("job not found: %s", jobID)
			}
			mu.Lock()
			if err := addJobSources(jobID, metadata.PID); err != nil {
				mu.Unlock()
				return err
			}
			knownJobs[jobID] = true
			mu.Unlock()

			// Start goroutine to detect when job stops
			go func() {
				for {
					time.Sleep(500 * time.Millisecond)
					mu.Lock()
					var stoppedJobs []string
					for jID, pid := range runningJobs {
						if !process.IsProcessRunning(pid) {
							stoppedJobs = append(stoppedJobs, jID)
						}
					}
					for _, jID := range stoppedJobs {
						follower.SystemLog("process stopped: %s", jID)
						delete(runningJobs, jID)
					}
					if len(stoppedJobs) > 0 && len(runningJobs) == 0 {
						follower.SystemLog("all processes stopped")
					}
					mu.Unlock()
				}
			}()
		}

		return follower.Wait()
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
}
