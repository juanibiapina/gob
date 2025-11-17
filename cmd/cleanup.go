package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/juanibiapina/job/internal/process"
	"github.com/juanibiapina/job/internal/storage"
	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove metadata for stopped jobs",
	Long:  `Remove metadata for stopped jobs. This command scans all job metadata files, checks if each process is still running, and removes metadata files for stopped processes only.`,
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

		// Count cleaned up jobs
		cleanedCount := 0

		// Process each job
		for _, job := range jobs {
			// Check if process is still running
			if !process.IsProcessRunning(job.Metadata.PID) {
				// Remove the metadata file
				filename := job.ID + ".json"
				filePath := filepath.Join(jobDir, filename)
				if err := os.Remove(filePath); err != nil {
					// Continue even if removal fails, but log the error
					fmt.Fprintf(os.Stderr, "Warning: failed to remove %s: %v\n", filename, err)
					continue
				}
				cleanedCount++
			}
		}

		// Print summary
		fmt.Printf("Cleaned up %d stopped job(s)\n", cleanedCount)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
}
