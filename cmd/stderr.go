package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/juanibiapina/gob/internal/storage"
	"github.com/juanibiapina/gob/internal/tail"
	"github.com/spf13/cobra"
)

var followStderr bool

var stderrCmd = &cobra.Command{
	Use:               "stderr <job_id>",
	Short:             "Display stderr output for a job",
	ValidArgsFunction: completeJobIDs,
	Long: `Display the entire stderr output for a background job.

Shows all output that the job has written to stderr since it started.
The output is read from the job's stderr log file.

Example:
  # View stderr for a job
  gob stderr V3x0QqI

Output:
  [Contents of stderr log file]

Notes:
  - Only works for jobs that have log files (jobs started with logging enabled)
  - Shows the complete output from the beginning
  - Old jobs started before logging was enabled will show an error

Exit codes:
  0: Output displayed successfully
  1: Error (job not found, log file not available)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobID := args[0]

		// Load job metadata to verify it exists
		_, err := storage.LoadJobMetadata(jobID + ".json")
		if err != nil {
			return fmt.Errorf("job not found: %s", jobID)
		}

		// Get job directory
		jobDir, err := storage.GetJobDir()
		if err != nil {
			return fmt.Errorf("failed to get job directory: %w", err)
		}

		// Calculate stderr file path using standard pattern
		stderrPath := filepath.Join(jobDir, fmt.Sprintf("%s.stderr.log", jobID))

		// Check if stderr file exists
		if _, err := os.Stat(stderrPath); os.IsNotExist(err) {
			return fmt.Errorf("stderr log file not found: %s", stderrPath)
		}

		// If follow flag is set, follow the log file in real-time
		if followStderr {
			return tail.Follow(stderrPath, os.Stdout)
		}

		// Read and display the stderr file
		content, err := os.ReadFile(stderrPath)
		if err != nil {
			return fmt.Errorf("failed to read stderr log: %w", err)
		}

		// Print the content (could be empty if no output yet)
		fmt.Print(string(content))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(stderrCmd)
	stderrCmd.Flags().BoolVarP(&followStderr, "follow", "f", false, "Follow log output in real-time")
}
