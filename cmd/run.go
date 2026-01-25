package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:                "run <command> [args...]",
	Short:              "Add a job and wait for it to complete",
	DisableFlagParsing: true,
	Long: `Add a new background job and immediately wait for it to complete.

This is equivalent to running 'gob add' followed by 'gob await'.

The job is started as a detached process and its output is streamed in real-time.
The command exits with the job's exit code when it completes.

Examples:
  # Run a build and wait for it
  gob run make build

  # Run tests
  gob run npm test

  # Quoted command strings also work
  gob run "make test"

  # Optional -- separator is also supported
  gob run -- npm run --flag

Output:
  Shows job statistics (if available), then streams the job's output.

Exit codes:
  Exits with the job's exit code (0 if successful, non-zero otherwise).
  Exits with 1 if there's an error (missing command, failed to start).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Handle missing arguments
		if len(args) == 0 {
			return fmt.Errorf("requires at least 1 arg(s)")
		}

		// Strip leading -- if present (optional separator)
		if args[0] == "--" {
			args = args[1:]
			if len(args) == 0 {
				return fmt.Errorf("requires at least 1 arg(s)")
			}
		}

		// Handle quoted command string: "echo hello world" -> ["echo", "hello", "world"]
		if len(args) == 1 && strings.Contains(args[0], " ") {
			args = strings.Fields(args[0])
		}

		// Connect to daemon
		client, err := daemon.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		defer client.Close()

		if err := client.Connect(); err != nil {
			return fmt.Errorf("failed to connect to daemon: %w", err)
		}

		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Capture current environment
		env := os.Environ()

		// Add job via daemon (no description for run command)
		result, err := client.Add(args, cwd, env, "")
		if err != nil {
			return fmt.Errorf("failed to add job: %w", err)
		}

		// Print confirmation message
		commandStr := strings.Join(args, " ")
		fmt.Printf("Running job %s: %s\n", result.Job.ID, commandStr)

		// Show stats if job has previous runs
		if result.Stats != nil && result.Stats.RunCount > 0 {
			fmt.Printf("  Previous runs: %d (%.0f%% success rate)\n",
				result.Stats.RunCount, result.Stats.SuccessRate)
			fmt.Printf("  Expected duration: ~%s\n",
				formatDuration(time.Duration(result.Stats.AvgDurationMs)*time.Millisecond))
		}

		// Follow the output until completion
		completed, err := followJob(result.Job.ID, result.Job.PID, result.Job.StdoutPath)
		if err != nil {
			return err
		}

		if !completed {
			fmt.Printf("\nJob %s continues running in background\n", result.Job.ID)
			fmt.Printf("  gob await %s   # wait for completion with live output\n", result.Job.ID)
			fmt.Printf("  gob stop %s    # stop the job\n", result.Job.ID)
			return nil
		}

		// Re-fetch job to get final state
		job, err := client.GetJob(result.Job.ID)
		if err != nil {
			return err
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

func init() {
	RootCmd.AddCommand(runCmd)
}
