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
	Long: `Start a stopped job with a new PID.

Retrieves the command from saved metadata and starts the process again.
The job ID remains the same, but a new PID is assigned.

Only works on stopped jobs - returns error if already running.

Example:
  # Start a stopped job
  job start 1732348944

Output:
  Started job <job_id> with new PID <pid> running: <command>

Notes:
  - Only works on stopped jobs
  - Preserves the job ID while updating the PID
  - Useful for restarting jobs that have stopped or crashed
  - The command is retrieved from saved metadata

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

		// Ensure job directory exists and get its path
		storageDir, err := storage.EnsureJobDir()
		if err != nil {
			return fmt.Errorf("failed to create job directory: %w", err)
		}

		pid, stdoutPath, stderrPath, err := process.StartDetached(command, commandArgs, metadata.ID, storageDir)
		if err != nil {
			return fmt.Errorf("failed to start job: %w", err)
		}

		// Update the PID and log file paths in metadata
		metadata.PID = pid
		metadata.StdoutFile = stdoutPath
		metadata.StderrFile = stderrPath

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
