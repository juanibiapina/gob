package cmd

import (
	"fmt"
	"time"

	"github.com/juanibiapina/gob/internal/process"
	"github.com/juanibiapina/gob/internal/storage"
	"github.com/spf13/cobra"
)

var forceStop bool

var stopCmd = &cobra.Command{
	Use:   "stop <job_id>",
	Short: "Stop a background job",
	Long: `Stop a background job by sending a signal to terminate it.

By default, sends SIGTERM for graceful shutdown.
Use --force to send SIGKILL for immediate termination.

Use 'job list' to find job IDs.

Examples:
  # Gracefully stop a job
  gob stop1732348944

  # Forcefully kill a stubborn job
  gob stop1732348944 --force

Output:
  Stopped job <job_id> (PID <pid>)

Or with --force:
  Force stopped job <job_id> (PID <pid>)

Notes:
  - Stopping an already-stopped job is not an error (idempotent)
  - Use --force if the job doesn't respond to SIGTERM
  - Job metadata is NOT removed (use 'job cleanup' or 'job remove')

Exit codes:
  0: Job stopped successfully (or already stopped)
  1: Error (job not found, failed to send signal)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobID := args[0]

		// Load job metadata
		metadata, err := storage.LoadJobMetadata(jobID + ".json")
		if err != nil {
			return fmt.Errorf("job not found: %s", jobID)
		}

		// Check if process is already stopped
		if !process.IsProcessRunning(metadata.PID) {
			if forceStop {
				fmt.Printf("Force stopped job %s (PID %d)\n", jobID, metadata.PID)
			} else {
				fmt.Printf("Stopped job %s (PID %d)\n", jobID, metadata.PID)
			}
			return nil
		}

		// Stop the process
		var stopErr error
		if forceStop {
			// Send SIGKILL immediately and wait for termination
			stopErr = process.KillProcess(metadata.PID)
			if stopErr != nil {
				return fmt.Errorf("failed to kill job %s: %w", jobID, stopErr)
			}

			// Poll for termination after SIGKILL
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				if !process.IsProcessRunning(metadata.PID) {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}

			// Verify process is actually dead
			if process.IsProcessRunning(metadata.PID) {
				return fmt.Errorf("process %d still running after SIGKILL", metadata.PID)
			}
		} else {
			// Use graceful shutdown with timeout
			stopErr = process.StopProcessWithTimeout(metadata.PID, 10*time.Second, 5*time.Second)
			if stopErr != nil {
				return fmt.Errorf("failed to stop job %s: %w", jobID, stopErr)
			}
		}

		// Print confirmation
		if forceStop {
			fmt.Printf("Force stopped job %s (PID %d)\n", jobID, metadata.PID)
		} else {
			fmt.Printf("Stopped job %s (PID %d)\n", jobID, metadata.PID)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
	stopCmd.Flags().BoolVarP(&forceStop, "force", "f", false, "Send SIGKILL instead of SIGTERM for forceful termination")
}
