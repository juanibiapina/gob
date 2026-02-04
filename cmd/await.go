package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var awaitCmd = &cobra.Command{
	Use:               "await <job_id>",
	Short:             "Wait for a job to complete and show its output",
	ValidArgsFunction: completeJobIDs,
	Long: `Wait for a job to complete, streaming its output in real-time.

For running jobs:
  - Streams stdout and stderr in real-time
  - Waits until the job completes
  - Shows a summary with exit code and duration

For stopped jobs:
  - Displays the complete stdout and stderr output
  - Shows a summary with exit code and duration

The job continues running in the background if you press Ctrl+C.

Examples:
  # Wait for job abc to complete
  gob await abc

Output:
  Shows the job's stdout and stderr, followed by a summary.

Exit codes:
  Exits with the job's exit code (0 if successful, non-zero otherwise).
  Exits with 1 if there's an error (job not found, connection failed).`,
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

		commandStr := strings.Join(job.Command, " ")

		if job.Status == "running" {
			// Fetch stats for stuck detection
			var avgDurationMs int64
			statsJob, err := client.Stats(jobID)
			if err == nil && statsJob != nil && statsJob.SuccessCount >= 3 {
				avgDurationMs = statsJob.AvgDurationMs
			}
			stuckTimeout := CalculateStuckTimeout(avgDurationMs)

			fmt.Printf("Awaiting job %s: %s\n", job.ID, commandStr)
			fmt.Printf("  Stuck detection: timeout after %s\n", formatDuration(stuckTimeout))

			// Follow the output until completion
			followResult, err := followJob(job.ID, job.PID, job.StdoutPath, avgDurationMs)
			if err != nil {
				return err
			}

			if followResult.PossiblyStuck {
				fmt.Printf("\nJob %s possibly stuck (no output for 1m)\n", job.ID)
				fmt.Printf("  gob stdout %s   # check current output\n", job.ID)
				fmt.Printf("  gob await %s    # continue waiting with output\n", job.ID)
				fmt.Printf("  gob stop %s     # stop the job\n", job.ID)
				return nil
			}

			if !followResult.Completed {
				fmt.Printf("\nJob %s continues running in background\n", job.ID)
				return nil
			}

			// Re-fetch job to get final state
			job, err = client.GetJob(jobID)
			if err != nil {
				return err
			}
		} else {
			// Job is stopped - show existing output
			fmt.Printf("Job %s (stopped): %s\n\n", job.ID, commandStr)

			if err := printJobOutput(job); err != nil {
				return err
			}
		}

		// Show summary
		printJobSummary(job)

		// Exit with job's exit code
		if job.ExitCode != nil && *job.ExitCode != 0 {
			os.Exit(*job.ExitCode)
		}

		return nil
	},
}

// printJobOutput prints the stdout and stderr of a stopped job
func printJobOutput(job *daemon.JobResponse) error {
	// Print stdout
	if _, err := os.Stat(job.StdoutPath); err == nil {
		content, err := os.ReadFile(job.StdoutPath)
		if err != nil {
			return fmt.Errorf("failed to read stdout: %w", err)
		}
		if len(content) > 0 {
			fmt.Print(string(content))
		}
	}

	// Print stderr
	stderrPath := strings.Replace(job.StdoutPath, ".stdout.log", ".stderr.log", 1)
	if _, err := os.Stat(stderrPath); err == nil {
		content, err := os.ReadFile(stderrPath)
		if err != nil {
			return fmt.Errorf("failed to read stderr: %w", err)
		}
		if len(content) > 0 {
			// Print stderr with yellow color
			fmt.Fprintf(os.Stderr, "\033[33m%s\033[0m", string(content))
		}
	}

	return nil
}

// printJobSummary prints a summary of the completed job
func printJobSummary(job *daemon.JobResponse) {
	fmt.Println()
	fmt.Printf("Job %s completed\n", job.ID)
	fmt.Printf("  Command:   %s\n", strings.Join(job.Command, " "))

	// Calculate and show duration
	if job.StartedAt != "" && job.StoppedAt != "" {
		startedAt, err1 := time.Parse(time.RFC3339, job.StartedAt)
		stoppedAt, err2 := time.Parse(time.RFC3339, job.StoppedAt)
		if err1 == nil && err2 == nil {
			duration := stoppedAt.Sub(startedAt)
			fmt.Printf("  Duration:  %s\n", formatDuration(duration))
		}
	}

	// Show exit code
	if job.ExitCode != nil {
		fmt.Printf("  Exit code: %d\n", *job.ExitCode)
	} else {
		fmt.Printf("  Exit code: unknown (killed by signal)\n")
	}
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", mins, secs)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", hours, mins)
}

func init() {
	RootCmd.AddCommand(awaitCmd)
}
