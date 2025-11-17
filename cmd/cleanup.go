package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/juanibiapina/gob/internal/process"
	"github.com/juanibiapina/gob/internal/storage"
	"github.com/spf13/cobra"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove metadata for stopped jobs",
	Long: `Remove metadata for all stopped jobs.

Scans all job metadata files and removes entries for stopped processes.
Leaves running jobs untouched.

Example:
  # Remove all stopped job metadata
  gob cleanup

Output:
  Cleaned up <n> stopped job(s)

Examples:
  Cleaned up 3 stopped job(s)

  Or if nothing to clean:
  Cleaned up 0 stopped job(s)

Notes:
  - Only removes metadata for processes that are no longer running
  - Does NOT stop any running jobs
  - Safe to run at any time
  - For removing a single job, use 'job remove <job_id>'

Exit codes:
  0: Cleanup completed successfully
  1: Error reading jobs`,
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
