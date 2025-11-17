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
	Long:  `List all background jobs with their PID, status (running/stopped), and command.`,
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
