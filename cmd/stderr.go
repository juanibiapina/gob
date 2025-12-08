package cmd

import (
	"fmt"
	"os"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/juanibiapina/gob/internal/tail"
	"github.com/spf13/cobra"
)

var followStderr bool

var stderrCmd = &cobra.Command{
	Use:               "stderr <job_id>",
	Short:             "Display stderr output for a job",
	ValidArgsFunction: completeJobIDs,
	Long: `Display the raw stderr output for a background job.

Shows all output that the job has written to stderr since it started.
The output is displayed exactly as written, without any prefixes or formatting.
Use the logs command instead for prefixed output with multiple streams.

Example:
  # View stderr for a job
  gob stderr V3x0QqI

  # Follow stderr in real-time
  gob stderr -f V3x0QqI

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

		// Connect to daemon
		client, err := daemon.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		defer client.Close()

		if err := client.Connect(); err != nil {
			return fmt.Errorf("failed to connect to daemon: %w", err)
		}

		// Get job from daemon
		job, err := client.GetJob(jobID)
		if err != nil {
			return err
		}

		stderrPath := job.StderrPath

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
	RootCmd.AddCommand(stderrCmd)
	stderrCmd.Flags().BoolVarP(&followStderr, "follow", "f", false, "Follow log output in real-time")
}
