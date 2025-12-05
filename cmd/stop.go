package cmd

import (
	"fmt"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var forceStop bool

var stopCmd = &cobra.Command{
	Use:               "stop <job_id>",
	Short:             "Stop a background job",
	ValidArgsFunction: completeJobIDs,
	Long: `Stop a background job by sending a signal to terminate it.

By default, sends SIGTERM for graceful shutdown.
Use --force to send SIGKILL for immediate termination.

Use 'job list' to find job IDs.

Examples:
  # Gracefully stop a job
  gob stop V3x0QqI

  # Forcefully kill a stubborn job
  gob stop V3x0QqI --force

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

		// Connect to daemon
		client, err := daemon.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		defer client.Close()

		if err := client.Connect(); err != nil {
			return fmt.Errorf("failed to connect to daemon: %w", err)
		}

		// Stop via daemon
		pid, err := client.Stop(jobID, forceStop)
		if err != nil {
			return fmt.Errorf("job not found: %s", jobID)
		}

		// Print confirmation
		if forceStop {
			fmt.Printf("Force stopped job %s (PID %d)\n", jobID, pid)
		} else {
			fmt.Printf("Stopped job %s (PID %d)\n", jobID, pid)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
	stopCmd.Flags().BoolVarP(&forceStop, "force", "f", false, "Send SIGKILL instead of SIGTERM for forceful termination")
}
