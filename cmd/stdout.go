package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/juanibiapina/job/internal/storage"
	"github.com/spf13/cobra"
)

var followStdout bool

var stdoutCmd = &cobra.Command{
	Use:   "stdout <job_id>",
	Short: "Display stdout output for a job",
	Long: `Display the entire stdout output for a background job.

Shows all output that the job has written to stdout since it started.
The output is read from the job's stdout log file.

Example:
  # View stdout for a job
  job stdout 1732348944

Output:
  [Contents of stdout log file]

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

		// Load job metadata
		metadata, err := storage.LoadJobMetadata(jobID + ".json")
		if err != nil {
			return fmt.Errorf("job not found: %s", jobID)
		}

		// Check if stdout file is configured
		if metadata.StdoutFile == "" {
			return fmt.Errorf("stdout log not available for job %s (job started before logging was enabled)", jobID)
		}

		// Check if stdout file exists
		if _, err := os.Stat(metadata.StdoutFile); os.IsNotExist(err) {
			return fmt.Errorf("stdout log file not found: %s", metadata.StdoutFile)
		}

		// If follow flag is set, use tail -f to follow the log file
		if followStdout {
			tailCmd := exec.Command("tail", "-f", metadata.StdoutFile)
			tailCmd.Stdout = os.Stdout
			tailCmd.Stderr = os.Stderr
			return tailCmd.Run()
		}

		// Read and display the stdout file
		content, err := os.ReadFile(metadata.StdoutFile)
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
	stdoutCmd.Flags().BoolVarP(&followStdout, "follow", "f", false, "Follow log output in real-time (like tail -f)")
}
