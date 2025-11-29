package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/juanibiapina/gob/internal/process"
	"github.com/juanibiapina/gob/internal/storage"
	"github.com/spf13/cobra"
)

var removeCmd = &cobra.Command{
	Use:               "remove <job_id>",
	Short:             "Remove metadata for a stopped job",
	ValidArgsFunction: completeJobIDs,
	Long: `Remove metadata for a single stopped job.

Only works on stopped jobs - returns an error if the job is still running.
Use 'job stop' first if needed.

For removing multiple stopped jobs at once, use 'job cleanup' instead.

Example:
  # Remove a specific stopped job
  gob remove 1732348944

Output:
  Removed job <job_id> (PID <pid>)

Notes:
  - Only works on stopped jobs (use 'job stop' first if needed)
  - For batch removal of stopped jobs, use 'job cleanup'
  - Unlike 'cleanup', removing a non-existent job returns an error

Exit codes:
  0: Job metadata removed successfully
  1: Error (job not found, job still running, failed to remove)`,
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
