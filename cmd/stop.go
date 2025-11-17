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
	Long: `Stop a background job by sending SIGTERM (or SIGKILL with --force).
The job ID can be obtained from the 'list' command.`,
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
