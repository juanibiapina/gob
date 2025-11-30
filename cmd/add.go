package cmd

import (
	"fmt"
	"strings"

	"github.com/juanibiapina/gob/internal/process"
	"github.com/juanibiapina/gob/internal/storage"
	"github.com/spf13/cobra"
)

var addFollow bool

var addCmd = &cobra.Command{
	Use:   "add <command> [args...]",
	Short: "Create and start a new background job",
	Long: `Create and start a new background job that continues running after the CLI exits.

The job is started as a detached process and assigned a unique job ID.
Use this ID with other commands to manage the job.

Use -- to separate gob flags from the command when the command has flags:
  gob add -- python -m http.server 8080

Examples:
  # Add a long-running sleep
  gob add sleep 3600

  # Add a server (use -- before commands with flags)
  gob add -- python -m http.server 8080

  # Add a background compilation
  gob add make build

  # Add and follow output until completion
  gob add -f -- make test

Output:
  Added job <job_id> running: <command>

Exit codes:
  0: Job added successfully
  1: Error (missing command, failed to start)`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// First argument is the command, rest are arguments
		command := args[0]
		commandArgs := []string{}
		if len(args) > 1 {
			commandArgs = args[1:]
		}

		// Generate job ID
		jobID := storage.GenerateJobID()

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
			return fmt.Errorf("failed to add job: %w", err)
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
		fmt.Printf("Added job %s running: %s\n", jobID, commandStr)

		// If follow flag is set, follow the output
		if addFollow {
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
	addCmd.Flags().BoolVarP(&addFollow, "follow", "f", false, "Follow output until job completes")
	rootCmd.AddCommand(addCmd)
}
