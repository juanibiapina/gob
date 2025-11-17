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

If the job is running, sends SIGTERM to stop it first.
If the job is already stopped, behaves like 'job start'.

The job ID remains the same, but a new PID is assigned.

Examples:
  # Restart a running job
  job restart 1732348944

  # Restart a stopped job (same as start)
  job restart 1732348944

Output:
  Restarted job <job_id> with new PID <pid> running: <command>

Notes:
  - Works on both running and stopped jobs
  - Uses SIGTERM for graceful shutdown (not SIGKILL)
  - Preserves the job ID while updating the PID
  - Useful for applying configuration changes or recovering from issues

Exit codes:
  0: Job restarted successfully
  1: Error (job not found, failed to stop/start)`,
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
		fmt.Printf("Restarted job %s with new PID %d running: %s\n", jobID, pid, commandStr)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(restartCmd)
}
