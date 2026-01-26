package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var restartFollow bool

var restartCmd = &cobra.Command{
	Use:               "restart <job_id>",
	Short:             "Restart a job (stop + start)",
	ValidArgsFunction: completeJobIDs,
	Long: `Restart a job by stopping it (if running) and starting it again.

If the job is running, sends SIGTERM to stop it first.
If the job is already stopped, simply starts it.

The job ID remains the same, but a new PID is assigned.

Examples:
  # Restart a running job
  gob restart V3x0QqI

  # Restart a stopped job (same as start)
  gob restart V3x0QqI

  # Restart and follow output until completion
  gob restart -f V3x0QqI

Output:
  Restarted job <job_id> with new PID <pid> running: <command>

Notes:
  - Works on both running and stopped jobs
  - Uses SIGTERM for graceful shutdown (not SIGKILL)
  - Preserves the job ID while updating the PID
  - Useful for applying configuration changes or recovering from issues

Exit codes:
  0: Job restarted successfully
  1: Error (job not found, failed to stop/start)`,
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

		// Restart via daemon
		job, err := client.Restart(jobID, env)
		if err != nil {
			return err
		}

		// Print confirmation message
		commandStr := strings.Join(job.Command, " ")
		fmt.Printf("Restarted job %s with new PID %d running: %s\n", jobID, job.PID, commandStr)

		// If follow flag is set, follow the output
		if restartFollow {
			// Fetch stats for stuck detection
			var avgDurationMs int64
			stats, statsErr := client.Stats(jobID)
			if statsErr == nil && stats != nil && stats.SuccessCount >= 3 {
				avgDurationMs = stats.AvgDurationMs
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
	restartCmd.Flags().BoolVarP(&restartFollow, "follow", "f", false, "Follow output until job completes")
	RootCmd.AddCommand(restartCmd)
}
