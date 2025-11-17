package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/juanibiapina/job/internal/process"
	"github.com/juanibiapina/job/internal/storage"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:   "remove <job_id>",
	Short: "Remove metadata for a stopped job",
	Long: `Remove metadata for a stopped job. This command removes the metadata file for a single job,
but only if the job is already stopped. If the job is still running, use 'stop' first.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobID := args[0]

		// Load job metadata
		metadata, err := storage.LoadJobMetadata(jobID + ".json")
		if err != nil {
			return fmt.Errorf("job not found: %s", jobID)
		}

		// Check if process is still running
		if process.IsProcessRunning(metadata.PID) {
			return fmt.Errorf("cannot remove running job: %s (use 'stop' first)", jobID)
		}

		// Get job directory
		jobDir, err := storage.GetJobDir()
		if err != nil {
			return fmt.Errorf("failed to get job directory: %w", err)
		}

		// Remove the metadata file
		filename := jobID + ".json"
		filePath := filepath.Join(jobDir, filename)
		if err := os.Remove(filePath); err != nil {
			return fmt.Errorf("failed to remove job metadata: %w", err)
		}

		// Print confirmation
		fmt.Printf("Removed job %s (PID %d)\n", jobID, metadata.PID)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
