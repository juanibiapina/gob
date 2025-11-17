package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/juanibiapina/job/internal/process"
	"github.com/juanibiapina/job/internal/storage"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:   "add <command> [args...]",
	Short: "Add a command as a background job",
	Long: `Add a command as a background job that continues running after the CLI exits.
The job metadata (command, PID, timestamp) is stored in .local/share/job/ for later reference.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// First argument is the command, rest are arguments
		command := args[0]
		commandArgs := []string{}
		if len(args) > 1 {
			commandArgs = args[1:]
		}

		// Generate job ID (Unix timestamp)
		jobID := time.Now().Unix()

		// Start the detached process
		pid, err := process.StartDetached(command, commandArgs)
		if err != nil {
			return fmt.Errorf("failed to start job: %w", err)
		}

		// Create job metadata
		metadata := &storage.JobMetadata{
			ID:      jobID,
			Command: args,
			PID:     pid,
		}

		// Save metadata
		_, err = storage.SaveJobMetadata(metadata)
		if err != nil {
			return fmt.Errorf("failed to save job metadata: %w", err)
		}

		// Print confirmation message
		commandStr := strings.Join(args, " ")
		fmt.Printf("Started job %d running: %s\n", jobID, commandStr)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}
