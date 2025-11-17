package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/juanibiapina/job/internal/storage"
	"github.com/spf13/cobra"
)

var followStderr bool

var stderrCmd = &cobra.Command{
	Use:   "stderr <job_id>",
	Short: "Display stderr output for a job",
	Long: `Display the entire stderr output for a background job.

Shows all output that the job has written to stderr since it started.
The output is read from the job's stderr log file.

Example:
  # View stderr for a job
  job stderr 1732348944

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

		// Load job metadata
		metadata, err := storage.LoadJobMetadata(jobID + ".json")
		if err != nil {
			return fmt.Errorf("job not found: %s", jobID)
		}

		// Check if stderr file is configured
		if metadata.StderrFile == "" {
			return fmt.Errorf("stderr log not available for job %s (job started before logging was enabled)", jobID)
		}

		// Check if stderr file exists
		if _, err := os.Stat(metadata.StderrFile); os.IsNotExist(err) {
			return fmt.Errorf("stderr log file not found: %s", metadata.StderrFile)
		}

		// If follow flag is set, use tail -f to follow the log file
		if followStderr {
			tailCmd := exec.Command("tail", "-f", metadata.StderrFile)
			tailCmd.Stdout = os.Stdout
			tailCmd.Stderr = os.Stderr
			return tailCmd.Run()
		}

		// Read and display the stderr file
		content, err := os.ReadFile(metadata.StderrFile)
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
	stderrCmd.Flags().BoolVarP(&followStderr, "follow", "f", false, "Follow log output in real-time (like tail -f)")
}
