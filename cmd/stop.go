package cmd

import (
	"fmt"

	"github.com/juanibiapina/job/internal/process"
	"github.com/juanibiapina/job/internal/storage"
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
  job stop 1732348944

  # Forcefully kill a stubborn job
  job stop 1732348944 --force

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
			stopErr = process.KillProcess(metadata.PID)
		} else {
			stopErr = process.StopProcess(metadata.PID)
		}

		if stopErr != nil {
			return fmt.Errorf("failed to stop job %s: %w", jobID, stopErr)
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
