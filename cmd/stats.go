package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var statsJSON bool

var statsCmd = &cobra.Command{
	Use:               "stats <job_id>",
	Short:             "Show statistics for a job",
	ValidArgsFunction: completeJobIDs,
	Long: `Show statistics for a job.

Displays aggregated statistics across all runs of the specified job, including:
- Total number of runs (successes, failures, and killed processes)
- Success rate (percentage of runs with exit code 0)
- Duration statistics for successes (average, minimum, maximum)
- Duration statistics for failures (average)

Example output:
  Job: abc (make test)
  Total runs: 10
  Success rate: 70% (7/10)
  Avg success duration: 2m30s
  Avg failure duration: 15s
  Fastest: 2m15s
  Slowest: 2m45s

With --json, outputs the full job response including statistics fields
(run_count, success_count, failure_count, success_rate, avg_duration_ms,
failure_avg_duration_ms, min_duration_ms, max_duration_ms) along with
all standard job fields (id, status, command, etc.).

Note: Statistics are calculated from completed runs only.
Running jobs and killed processes are excluded from duration averages.
Killed processes (sent SIGTERM/SIGKILL) still count toward total runs but
not toward success/failure counts or duration statistics.

Exit codes:
  0: Success
  1: Error (job not found)`,
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

		// Get stats from daemon
		job, err := client.Stats(jobID)
		if err != nil {
			return err
		}

		// Output as JSON or human-readable
		if statsJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(job)
		}

		// Print stats in human-readable format
		commandStr := strings.Join(job.Command, " ")
		fmt.Printf("Job: %s (%s)\n", job.ID, commandStr)

		if job.RunCount == 0 {
			fmt.Println("No completed runs yet")
			return nil
		}

		fmt.Printf("Total runs: %d\n", job.RunCount)
		fmt.Printf("Success rate: %.0f%% (%d/%d)\n", job.SuccessRate, job.SuccessCount, job.RunCount)
		if job.SuccessCount > 0 {
			fmt.Printf("Avg success duration: %s\n", formatDuration(time.Duration(job.AvgDurationMs)*time.Millisecond))
		}
		if job.FailureCount > 0 {
			fmt.Printf("Avg failure duration: %s\n", formatDuration(time.Duration(job.FailureAvgDurationMs)*time.Millisecond))
		}
		fmt.Printf("Fastest: %s\n", formatDuration(time.Duration(job.MinDurationMs)*time.Millisecond))
		fmt.Printf("Slowest: %s\n", formatDuration(time.Duration(job.MaxDurationMs)*time.Millisecond))

		return nil
	},
}

func init() {
	RootCmd.AddCommand(statsCmd)
	statsCmd.Flags().BoolVar(&statsJSON, "json", false, "Output in JSON format")
}
