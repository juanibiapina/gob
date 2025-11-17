package cmd

import (
	"fmt"
	"strings"

	"github.com/juanibiapina/job/internal/process"
	"github.com/juanibiapina/job/internal/storage"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start <job_id>",
	Short: "Start a stopped job",
	Long: `Start a stopped job using its saved command.
The job ID can be obtained from the 'list' command.
This will start the job with a new PID while keeping the same job ID.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobID := args[0]

		// Load job metadata
		metadata, err := storage.LoadJobMetadata(jobID + ".json")
		if err != nil {
			return fmt.Errorf("job not found: %s", jobID)
		}

		// Check if process is already running
		if process.IsProcessRunning(metadata.PID) {
			return fmt.Errorf("job %s is already running (PID %d)", jobID, metadata.PID)
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
		fmt.Printf("Started job %s with new PID %d running: %s\n", jobID, pid, commandStr)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}
