package cmd

import (
	"fmt"
	"strings"

	"github.com/juanibiapina/job/internal/process"
	"github.com/juanibiapina/job/internal/storage"
	"github.com/spf13/cobra"
)

var restartCmd = &cobra.Command{
	Use:   "restart <job_id>",
	Short: "Restart a job",
	Long: `Restart a job by stopping it (if running) and starting it again.
The job ID can be obtained from the 'list' command.
This will restart the job with a new PID while keeping the same job ID.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobID := args[0]

		// Load job metadata
		metadata, err := storage.LoadJobMetadata(jobID + ".json")
		if err != nil {
			return fmt.Errorf("job not found: %s", jobID)
		}

		// If process is running, stop it first
		if process.IsProcessRunning(metadata.PID) {
			err := process.StopProcess(metadata.PID)
			if err != nil {
				return fmt.Errorf("failed to stop job %s: %w", jobID, err)
			}
			// Give the process a moment to terminate
			// TODO: Consider adding a wait loop with timeout
		}

		// Start the process with the saved command
		command := metadata.Command[0]
		commandArgs := []string{}
		if len(metadata.Command) > 1 {
			commandArgs = metadata.Command[1:]
		}

		pid, err := process.StartDetached(command, commandArgs)
		if err != nil {
			return fmt.Errorf("failed to start job: %w", err)
		}

		// Update the PID in metadata
		metadata.PID = pid

		// Save updated metadata
		_, err = storage.SaveJobMetadata(metadata)
		if err != nil {
			return fmt.Errorf("failed to update job metadata: %w", err)
		}

		// Print confirmation message
		commandStr := strings.Join(metadata.Command, " ")
		fmt.Printf("Restarted job %s with new PID %d running: %s\n", jobID, pid, commandStr)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)
}
