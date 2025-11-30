package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/juanibiapina/gob/internal/process"
	"github.com/juanibiapina/gob/internal/storage"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:                "run <command> [args...]",
	Short:              "Run a command and follow its output until completion",
	DisableFlagParsing: true,
	Long: `Run a command as a background job and follow its output until completion.

Smart job reuse:
  - If a job with the same command+args exists and is stopped → restart + follow
  - If a job with the same command+args exists and is running → error
  - If no matching job exists → create new job + follow

The job runs in the background, so if you press Ctrl+C or kill the CLI,
the job continues running normally.

Examples:
  # Run a build and follow its output
  gob run make build

  # Run tests (will reuse existing job if found)
  gob run make test

  # Run a command with flags (no special handling needed)
  gob run ls -la
  gob run pnpm --filter web typecheck

  # Quoted command strings also work
  gob run "make test"

  # Optional -- separator is also supported
  gob run -- python -m http.server 8080

Output:
  Shows the job's stdout and stderr in real-time until the job completes.

Exit codes:
  0: Job completed
  1: Error (failed to start job, job already running)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Handle missing arguments
		if len(args) == 0 {
			return fmt.Errorf("requires at least 1 arg(s)")
		}

		// Strip leading -- if present (optional separator)
		if args[0] == "--" {
			args = args[1:]
			if len(args) == 0 {
				return fmt.Errorf("requires at least 1 arg(s)")
			}
		}

		// Handle quoted command string: "echo hello world" -> ["echo", "hello", "world"]
		if len(args) == 1 && strings.Contains(args[0], " ") {
			args = strings.Fields(args[0])
		}
		// Check if a job with the same command exists
		existingJob, err := storage.FindJobByCommand(args)
		if err != nil {
			return fmt.Errorf("failed to search for existing job: %w", err)
		}

		var jobID string
		var pid int
		var storageDir string

		if existingJob != nil {
			// Found existing job
			jobID = existingJob.ID

			// Check if it's running
			if process.IsProcessRunning(existingJob.PID) {
				return fmt.Errorf("job %s is already running with the same command", jobID)
			}

			// Clear previous logs before restarting
			if err := storage.ClearJobLogs(jobID); err != nil {
				return fmt.Errorf("failed to clear previous logs: %w", err)
			}

			// Restart the stopped job
			command := existingJob.Command[0]
			commandArgs := []string{}
			if len(existingJob.Command) > 1 {
				commandArgs = existingJob.Command[1:]
			}

			storageDir, err = storage.EnsureJobDir()
			if err != nil {
				return fmt.Errorf("failed to create job directory: %w", err)
			}

			pid, err = process.StartDetached(command, commandArgs, existingJob.ID, storageDir)
			if err != nil {
				return fmt.Errorf("failed to restart job: %w", err)
			}

			// Update the PID in metadata
			existingJob.PID = pid
			_, err = storage.SaveJobMetadata(existingJob)
			if err != nil {
				return fmt.Errorf("failed to update job metadata: %w", err)
			}

			commandStr := strings.Join(existingJob.Command, " ")
			fmt.Printf("Restarted job %s running: %s\n", jobID, commandStr)
		} else {
			// Create new job
			command := args[0]
			commandArgs := []string{}
			if len(args) > 1 {
				commandArgs = args[1:]
			}

			jobID = storage.GenerateJobID()

			cwd, err := storage.GetCurrentWorkdir()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}

			storageDir, err = storage.EnsureJobDir()
			if err != nil {
				return fmt.Errorf("failed to create job directory: %w", err)
			}

			pid, err = process.StartDetached(command, commandArgs, jobID, storageDir)
			if err != nil {
				return fmt.Errorf("failed to add job: %w", err)
			}

			metadata := &storage.JobMetadata{
				ID:        jobID,
				Command:   args,
				PID:       pid,
				Workdir:   cwd,
				CreatedAt: time.Now(),
			}

			_, err = storage.SaveJobMetadata(metadata)
			if err != nil {
				return fmt.Errorf("failed to save job metadata: %w", err)
			}

			commandStr := strings.Join(args, " ")
			fmt.Printf("Added job %s running: %s\n", jobID, commandStr)
		}

		// Small delay to let process start
		time.Sleep(50 * time.Millisecond)

		// Follow the output
		completed, err := followJob(jobID, pid, storageDir)
		if err != nil {
			return err
		}

		if completed {
			fmt.Printf("\nJob %s completed\n", jobID)
		} else {
			fmt.Printf("\nJob %s continues running in background\n", jobID)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
