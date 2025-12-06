package cmd

import (
	"fmt"
	"os"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var nukeAll bool

var nukeCmd = &cobra.Command{
	Use:   "nuke",
	Short: "Stop all running jobs and remove all job data",
	Long: `Stop all running jobs and remove all job data including logs.

⚠️  DESTRUCTIVE COMMAND - stops ALL jobs and removes ALL data.

By default, only affects jobs in the current directory.
Use --all to nuke jobs from all directories.

Workflow:
  1. Sends SIGTERM to all running jobs
  2. Removes all log files (stdout and stderr)
  3. Removes all jobs (both running and stopped)

Example:
  # Stop everything in current directory and start fresh
  gob nuke

  # Stop everything from all directories
  gob nuke --all

Output:
  Stopped <n> running job(s)
  Deleted <m> log file(s)
  Cleaned up <k> total job(s)

Example output:
  Stopped 2 running job(s)
  Deleted 4 log file(s)
  Cleaned up 2 total job(s)

Notes:
  - Uses SIGTERM (graceful) not SIGKILL
  - If jobs don't respond to SIGTERM, use 'job stop --force' individually first
  - Useful for cleaning up test environments or complete resets
  - Cannot be undone - all job data will be lost

Exit codes:
  0: Nuke completed successfully
  1: Error (failed to read jobs, failed to stop some jobs)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Connect to daemon
		client, err := daemon.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		defer client.Close()

		if err := client.Connect(); err != nil {
			return fmt.Errorf("failed to connect to daemon: %w", err)
		}

		// Determine workdir filter
		var workdirFilter string
		if !nukeAll {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
			workdirFilter = cwd
		}

		// Nuke via daemon
		stopped, logsDeleted, cleaned, err := client.Nuke(workdirFilter)
		if err != nil {
			return fmt.Errorf("failed to nuke: %w", err)
		}

		// Print summary
		fmt.Printf("Stopped %d running job(s)\n", stopped)
		fmt.Printf("Deleted %d log file(s)\n", logsDeleted)
		fmt.Printf("Cleaned up %d total job(s)\n", cleaned)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(nukeCmd)
	nukeCmd.Flags().BoolVarP(&nukeAll, "all", "a", false,
		"Nuke jobs from all directories")
}
