package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/juanibiapina/gob/internal/process"
	"github.com/juanibiapina/gob/internal/storage"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start <command> [args...]",
	Short: "Start a command as a background job",
	Long: `Start a command as a background job that continues running after the CLI exits.

The job is started as a detached process and assigned a unique job ID (Unix timestamp in nanoseconds).
Use this ID with other commands to manage the job.

Examples:
  # Start a long-running sleep
  gob start sleep 3600

  # Start a server
  gob start python -m http.server 8080

  # Start a background compilation
  gob start make build

Output:
  Started job <job_id> running: <command>

Exit codes:
  0: Job started successfully
  1: Error (missing command, failed to start)`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// First argument is the command, rest are arguments
		command := args[0]
		commandArgs := []string{}
		if len(args) > 1 {
			commandArgs = args[1:]
		}

		// Generate job ID (Unix timestamp in nanoseconds)
		jobID := time.Now().UnixNano()

		// Get current working directory
		cwd, err := storage.GetCurrentWorkdir()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Ensure job directory exists and get its path
		storageDir, err := storage.EnsureJobDir()
		if err != nil {
			return fmt.Errorf("failed to create job directory: %w", err)
		}

		// Start the detached process
		pid, err := process.StartDetached(command, commandArgs, jobID, storageDir)
		if err != nil {
			return fmt.Errorf("failed to start job: %w", err)
		}

		// Create job metadata
		metadata := &storage.JobMetadata{
			ID:      jobID,
			Command: args,
			PID:     pid,
			Workdir: cwd,
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
	rootCmd.AddCommand(startCmd)
}
