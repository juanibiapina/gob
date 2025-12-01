package cmd

import (
	"fmt"
	"strings"

	"github.com/juanibiapina/gob/internal/process"
	"github.com/juanibiapina/gob/internal/storage"
	"github.com/spf13/cobra"
)

var startFollow bool

var startCmd = &cobra.Command{
	Use:               "start <job_id>",
	Short:             "Start a stopped job",
	ValidArgsFunction: completeJobIDs,
	Long: `Start a stopped job by its job ID.

The job must be stopped (not running). If you want to restart a running job,
use 'gob restart' instead.

Examples:
  # Start a stopped job
  gob start V3x0QqI

  # Start and follow output until completion
  gob start -f V3x0QqI

Output:
  Started job <job_id> with PID <pid> running: <command>

Exit codes:
  0: Job started successfully
  1: Error (job not found, job already running, failed to start)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobID := args[0]

		// Load job metadata
		metadata, err := storage.LoadJobMetadata(jobID + ".json")
		if err != nil {
			return fmt.Errorf("job not found: %s", jobID)
		}

		// Check if job is already running
		if process.IsProcessRunning(metadata.PID) {
			return fmt.Errorf("job %s is already running (use 'gob restart' to restart a running job)", jobID)
		}

		// Start the process with the saved command
		command := metadata.Command[0]
		commandArgs := []string{}
		if len(metadata.Command) > 1 {
			commandArgs = metadata.Command[1:]
		}

		// Ensure job directory exists and get its path
		storageDir, err := storage.EnsureJobDir()
		if err != nil {
			return fmt.Errorf("failed to create job directory: %w", err)
		}

		// Clear previous logs before starting
		if err := storage.ClearJobLogs(jobID); err != nil {
			return fmt.Errorf("failed to clear job logs: %w", err)
		}

		pid, err := process.StartDetached(command, commandArgs, metadata.ID, storageDir)
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
		fmt.Printf("Started job %s with PID %d running: %s\n", jobID, pid, commandStr)

		// If follow flag is set, follow the output
		if startFollow {
			completed, err := followJob(jobID, pid, storageDir)
			if err != nil {
				return err
			}
			if completed {
				fmt.Printf("\nJob %s completed\n", jobID)
			} else {
				fmt.Printf("\nJob %s continues running in background\n", jobID)
			}
		}

		return nil
	},
}

func init() {
	startCmd.Flags().BoolVarP(&startFollow, "follow", "f", false, "Follow output until job completes")
	rootCmd.AddCommand(startCmd)
}
