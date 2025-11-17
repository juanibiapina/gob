package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/juanibiapina/job/internal/process"
	"github.com/juanibiapina/job/internal/storage"
	"github.com/spf13/cobra"
)

var nukeCmd = &cobra.Command{
	Use:   "nuke",
	Short: "Stop all running jobs and remove all job metadata",
	Long:  `Stop all running jobs and remove all job metadata. This is a destructive command that sends SIGTERM to all running jobs and removes all metadata files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get all jobs
		jobs, err := storage.ListJobMetadata()
		if err != nil {
			return fmt.Errorf("failed to list jobs: %w", err)
		}

		// Get job directory
		jobDir, err := storage.GetJobDir()
		if err != nil {
			return fmt.Errorf("failed to get job directory: %w", err)
		}

		// Count stopped and cleaned jobs
		stoppedCount := 0
		cleanedCount := 0

		// First pass: stop all running jobs
		for _, job := range jobs {
			// Check if process is still running
			if process.IsProcessRunning(job.Metadata.PID) {
				// Stop the process with SIGTERM
				if err := process.StopProcess(job.Metadata.PID); err != nil {
					// Log error but continue with other jobs
					fmt.Fprintf(os.Stderr, "Warning: failed to stop job %s (PID %d): %v\n", job.ID, job.Metadata.PID, err)
					continue
				}
				stoppedCount++
			}
		}

		// Second pass: remove all metadata files
		for _, job := range jobs {
			filename := job.ID + ".json"
			filePath := filepath.Join(jobDir, filename)
			if err := os.Remove(filePath); err != nil {
				// Log error but continue with other jobs
				fmt.Fprintf(os.Stderr, "Warning: failed to remove %s: %v\n", filename, err)
				continue
			}
			cleanedCount++
		}

		// Print summary
		fmt.Printf("Stopped %d running job(s)\n", stoppedCount)
		fmt.Printf("Cleaned up %d total job(s)\n", cleanedCount)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(nukeCmd)
}
