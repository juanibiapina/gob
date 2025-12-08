package cmd

import (
	"fmt"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:               "remove <job_id>",
	Short:             "Remove a stopped job",
	ValidArgsFunction: completeJobIDs,
	Long: `Remove a single stopped job.

Only works on stopped jobs - returns an error if the job is still running.
Use 'job stop' first if needed.

For removing multiple stopped jobs at once, use 'job cleanup' instead.

Example:
  # Remove a specific stopped job
  gob remove V3x0QqI

Output:
  Removed job <job_id> (PID <pid>)

Notes:
  - Only works on stopped jobs (use 'job stop' first if needed)
  - For batch removal of stopped jobs, use 'job cleanup'
  - Unlike 'cleanup', removing a non-existent job returns an error

Exit codes:
  0: Job removed successfully
  1: Error (job not found, job still running, failed to remove)`,
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

		// Remove via daemon
		pid, err := client.Remove(jobID)
		if err != nil {
			return err
		}

		// Print confirmation
		fmt.Printf("Removed job %s (PID %d)\n", jobID, pid)

		return nil
	},
}

func init() {
	RootCmd.AddCommand(removeCmd)
}
