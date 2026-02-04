package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var startFollow bool

var startCmd = &cobra.Command{
	Use:               "start <job_id>",
	Short:             "Start a stopped job",
	ValidArgsFunction: completeJobIDs,
	Long: `Start a stopped job by its job ID.

The job must be stopped (not running). If you want to restart a running job,
use 'gob restart' instead.

Examples:
  # Start a stopped job
  gob start V3x0QqI

  # Start and follow output until completion
  gob start -f V3x0QqI

Output:
  Started job <job_id> with PID <pid> running: <command>

Exit codes:
  0: Job started successfully
  1: Error (job not found, job already running, failed to start)`,
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

		// Capture current environment
		env := os.Environ()

		// Start via daemon
		job, err := client.Start(jobID, env)
		if err != nil {
			return err
		}

		// Print confirmation message
		commandStr := strings.Join(job.Command, " ")
		fmt.Printf("Started job %s with PID %d running: %s\n", jobID, job.PID, commandStr)

		// If follow flag is set, follow the output
		if startFollow {
			// Fetch stats for stuck detection
			var avgDurationMs int64
			statsJob, statsErr := client.Stats(jobID)
			if statsErr == nil && statsJob != nil && statsJob.SuccessCount >= 3 {
				avgDurationMs = statsJob.AvgDurationMs
			}
			stuckTimeout := CalculateStuckTimeout(avgDurationMs)
			fmt.Printf("  Stuck detection: timeout after %s\n", formatDuration(stuckTimeout))

			followResult, err := followJob(jobID, job.PID, job.StdoutPath, avgDurationMs)
			if err != nil {
				return err
			}
			if followResult.PossiblyStuck {
				fmt.Printf("\nJob %s possibly stuck (no output for 1m)\n", jobID)
				fmt.Printf("  gob stdout %s   # check current output\n", jobID)
				fmt.Printf("  gob await %s    # continue waiting with output\n", jobID)
				fmt.Printf("  gob stop %s     # stop the job\n", jobID)
			} else if followResult.Completed {
				fmt.Printf("\nJob %s completed\n", jobID)
			} else {
				fmt.Printf("\nJob %s continues running in background\n", jobID)
			}
		}

		return nil
	},
}

func init() {
	startCmd.Flags().BoolVarP(&startFollow, "follow", "f", false, "Follow output until job completes")
	RootCmd.AddCommand(startCmd)
}
