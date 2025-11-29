package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/juanibiapina/gob/internal/storage"
	"github.com/juanibiapina/gob/internal/tail"
	"github.com/spf13/cobra"
)

var followStdout bool

var stdoutCmd = &cobra.Command{
	Use:               "stdout <job_id>",
	Short:             "Display stdout output for a job",
	ValidArgsFunction: completeJobIDs,
	Long: `Display the raw stdout output for a background job.

Shows all output that the job has written to stdout since it started.
The output is displayed exactly as written, without any prefixes or formatting.
Use the logs command instead for prefixed output with multiple streams.

Example:
  # View stdout for a job
  gob stdout V3x0QqI

  # Follow stdout in real-time
  gob stdout -f V3x0QqI

Notes:
  - Output is raw with no prefixes (unlike the logs command)
  - Shows the complete output from the beginning
  - Use -f/--follow to stream output in real-time

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

		// Calculate stdout file path using standard pattern
		stdoutPath := filepath.Join(jobDir, fmt.Sprintf("%s.stdout.log", jobID))

		// Check if stdout file exists
		if _, err := os.Stat(stdoutPath); os.IsNotExist(err) {
			return fmt.Errorf("stdout log file not found: %s", stdoutPath)
		}

		// If follow flag is set, follow the log file in real-time
		if followStdout {
			return tail.Follow(stdoutPath, os.Stdout)
		}

		// Read and display the stdout file
		content, err := os.ReadFile(stdoutPath)
		if err != nil {
			return fmt.Errorf("failed to read stdout log: %w", err)
		}

		// Print the content (could be empty if no output yet)
		fmt.Print(string(content))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(stdoutCmd)
	stdoutCmd.Flags().BoolVarP(&followStdout, "follow", "f", false, "Follow log output in real-time")
}
