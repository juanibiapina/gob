package cmd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var runsJSON bool

var runsCmd = &cobra.Command{
	Use:               "runs <job_id>",
	Short:             "Show run history for a job",
	ValidArgsFunction: completeJobIDs,
	Long: `Show the run history for a job.

Displays all runs for the specified job, sorted by start time (newest first).
Each run shows its ID, when it started, duration, and exit status.

Output format:
  <run_id>  <started>  <duration>  <status>

Where:
  run_id:   Internal run identifier (e.g., abc-1, abc-2)
  started:  When the run started (relative time or timestamp)
  duration: How long the run took (or "running" if still active)
  status:   Exit status: ◉ (running), ✓ (0) for success, ✗ (N) for failure

Example output:
  abc-5  2 min ago   running   ◉
  abc-4  1 hour ago  2m15s     ✓ (0)
  abc-3  2 hours ago 2m45s     ✗ (1)

Subcommands:
  runs delete <run_id>  Delete a stopped run and its log files

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

		// Get runs from daemon
		runs, err := client.Runs(jobID)
		if err != nil {
			return err
		}

		// If no runs, print message (unless JSON output)
		if len(runs) == 0 {
			if runsJSON {
				fmt.Println("[]")
			} else {
				fmt.Println("No runs found")
			}
			return nil
		}

		// Output as JSON or human-readable
		if runsJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(runs)
		}

		// Print each run in human-readable format
		for _, run := range runs {
			startedAt, _ := time.Parse("2006-01-02T15:04:05Z07:00", run.StartedAt)
			started := formatRelativeTime(startedAt)

			var duration string
			var status string

			if run.Status == "running" {
				duration = "running"
				status = "◉"
			} else {
				duration = formatDuration(time.Duration(run.DurationMs) * time.Millisecond)
				if run.ExitCode != nil {
					if *run.ExitCode == 0 {
						status = fmt.Sprintf("✓ (%d)", *run.ExitCode)
					} else {
						status = fmt.Sprintf("✗ (%d)", *run.ExitCode)
					}
				} else {
					status = "✗ (killed)"
				}
			}

			fmt.Printf("%s  %-12s  %-10s  %s\n", run.ID, started, duration, status)
		}

		return nil
	},
}

var runsDeleteCmd = &cobra.Command{
	Use:   "delete <run_id>",
	Short: "Delete a stopped run and its log files",
	Long: `Delete a stopped run and its associated log files.

The run must be stopped (not currently running). To delete a running run,
first stop the job with 'gob stop <job_id>'.

Examples:
  gob runs delete abc-1
  gob runs delete myserver-5

Exit codes:
  0: Success
  1: Error (run not found, run still running)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		runID := args[0]

		// Connect to daemon
		client, err := daemon.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		defer client.Close()

		if err := client.Connect(); err != nil {
			return fmt.Errorf("failed to connect to daemon: %w", err)
		}

		// Delete the run
		if err := client.RemoveRun(runID); err != nil {
			return err
		}

		fmt.Printf("Deleted run %s\n", runID)
		return nil
	},
}

// formatRelativeTime formats a time as a human-readable relative string
func formatRelativeTime(t time.Time) string {
	d := time.Since(t)

	if d < time.Minute {
		return "just now"
	} else if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 min ago"
		}
		return fmt.Sprintf("%d mins ago", mins)
	} else if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

func init() {
	RootCmd.AddCommand(runsCmd)
	runsCmd.Flags().BoolVar(&runsJSON, "json", false, "Output in JSON format")
	runsCmd.AddCommand(runsDeleteCmd)
}
