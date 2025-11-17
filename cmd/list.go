package cmd

import (
	"fmt"
	"strings"

	"github.com/juanibiapina/job/internal/process"
	"github.com/juanibiapina/job/internal/storage"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all background jobs",
	Long: `List all background jobs with their current status.

Shows job ID, PID, status (running/stopped), and the original command.
Jobs are sorted by start time (newest first).

Output format:
  <job_id>: [<pid>] <status>: <command>

Where:
  job_id: Unique identifier (Unix timestamp) - use this for other commands
  pid:    Process ID
  status: Either 'running' or 'stopped'
  command: Original command that was executed

Example output:
  1732350000: [12345] running: sleep 3600
  1732349000: [12344] stopped: python server.py
  1732348000: [12343] running: make watch

If no jobs exist:
  No jobs found

Exit codes:
  0: Success
  1: Error reading jobs`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get all jobs
		jobs, err := storage.ListJobMetadata()
		if err != nil {
			return fmt.Errorf("failed to list jobs: %w", err)
		}

		// If no jobs, print message
		if len(jobs) == 0 {
			fmt.Println("No jobs found")
			return nil
		}

		// Print each job
		for _, job := range jobs {
			// Check if process is running
			var status string
			if process.IsProcessRunning(job.Metadata.PID) {
				status = "running"
			} else {
				status = "stopped"
			}

			// Format command as a single string
			commandStr := strings.Join(job.Metadata.Command, " ")

			// Print in format: <job_id>: [<pid>] <status>: <command>
			fmt.Printf("%s: [%d] %s: %s\n", job.ID, job.Metadata.PID, status, commandStr)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
