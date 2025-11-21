package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/juanibiapina/gob/internal/process"
	"github.com/juanibiapina/gob/internal/storage"
	"github.com/spf13/cobra"
)

var nukeAll bool

var nukeCmd = &cobra.Command{
	Use:   "nuke",
	Short: "Stop all running jobs and remove all job data",
	Long: `Stop all running jobs and remove all job data including logs and metadata.

⚠️  DESTRUCTIVE COMMAND - stops ALL jobs and removes ALL data.

By default, only affects jobs in the current directory.
Use --all to nuke jobs from all directories.

Workflow:
  1. Sends SIGTERM to all running jobs
  2. Removes all log files (stdout and stderr)
  3. Removes all metadata files (both running and stopped)

Example:
  # Stop everything in current directory and start fresh
  gob nuke

  # Stop everything from all directories
  gob nuke --all

Output:
  Stopped <n> running job(s)
  Deleted <m> log file(s)
  Cleaned up <k> total job(s)

Example output:
  Stopped 2 running job(s)
  Deleted 4 log file(s)
  Cleaned up 2 total job(s)

Notes:
  - Uses SIGTERM (graceful) not SIGKILL
  - If jobs don't respond to SIGTERM, use 'job stop --force' individually first
  - Useful for cleaning up test environments or complete resets
  - Cannot be undone - all job metadata will be lost

Exit codes:
  0: Nuke completed successfully
  1: Error (failed to read jobs, failed to stop some jobs)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var jobs []storage.JobInfo
		var err error

		// Get jobs based on --all flag
		if nukeAll {
			jobs, err = storage.ListAllJobMetadata()
		} else {
			jobs, err = storage.ListJobMetadata()
		}

		if err != nil {
			return fmt.Errorf("failed to list jobs: %w", err)
		}

		// Get job directory
		jobDir, err := storage.GetJobDir()
		if err != nil {
			return fmt.Errorf("failed to get job directory: %w", err)
		}

		// Count stopped, log files deleted, and cleaned jobs
		stoppedCount := 0
		logFilesDeleted := 0
		cleanedCount := 0

		// First pass: stop all running jobs with timeout
		for _, job := range jobs {
			// Check if process is still running
			if process.IsProcessRunning(job.Metadata.PID) {
				// Stop the process with timeout (10s graceful, 5s force)
				if err := process.StopProcessWithTimeout(job.Metadata.PID, 10*time.Second, 5*time.Second); err != nil {
					// Log error but continue with other jobs
					fmt.Fprintf(os.Stderr, "Warning: failed to stop job %s (PID %d): %v\n", job.ID, job.Metadata.PID, err)
					continue
				}
				stoppedCount++
			}
		}

		// Second pass: delete log files
		for _, job := range jobs {
			// Delete stdout log file
			stdoutPath := filepath.Join(jobDir, fmt.Sprintf("%s.stdout.log", job.ID))
			if err := os.Remove(stdoutPath); err != nil && !os.IsNotExist(err) {
				// Log error but continue with other files
				fmt.Fprintf(os.Stderr, "Warning: failed to remove stdout log %s: %v\n", stdoutPath, err)
			} else if err == nil {
				logFilesDeleted++
			}

			// Delete stderr log file
			stderrPath := filepath.Join(jobDir, fmt.Sprintf("%s.stderr.log", job.ID))
			if err := os.Remove(stderrPath); err != nil && !os.IsNotExist(err) {
				// Log error but continue with other files
				fmt.Fprintf(os.Stderr, "Warning: failed to remove stderr log %s: %v\n", stderrPath, err)
			} else if err == nil {
				logFilesDeleted++
			}
		}

		// Third pass: remove all metadata files
		for _, job := range jobs {
			filename := job.ID + ".json"
			filePath := filepath.Join(jobDir, filename)
			if err := os.Remove(filePath); err != nil {
				// Log error but continue with other jobs
				fmt.Fprintf(os.Stderr, "Warning: failed to remove %s: %v\n", filename, err)
				continue
			}
			cleanedCount++
		}

		// Print summary
		fmt.Printf("Stopped %d running job(s)\n", stoppedCount)
		fmt.Printf("Deleted %d log file(s)\n", logFilesDeleted)
		fmt.Printf("Cleaned up %d total job(s)\n", cleanedCount)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(nukeCmd)
	nukeCmd.Flags().BoolVarP(&nukeAll, "all", "a", false,
		"Nuke jobs from all directories")
}
