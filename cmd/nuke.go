package cmd

import (
	"fmt"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var nukeCmd = &cobra.Command{
	Use:   "nuke",
	Short: "Stop all jobs, remove all data, and shutdown daemon",
	Long: `Stop all running jobs, remove all job data, and shutdown the daemon.

⚠️  DESTRUCTIVE COMMAND - complete reset of gob.

Workflow:
  1. Sends SIGTERM to all running jobs
  2. Removes all log files (stdout and stderr)
  3. Removes all jobs (both running and stopped)
  4. Shuts down the daemon

Example:
  gob nuke

Output:
  Stopped <n> running job(s)
  Deleted <m> log file(s)
  Cleaned up <k> total job(s)
  Daemon shut down

Notes:
  - Uses SIGTERM (graceful) not SIGKILL
  - Affects ALL jobs from ALL directories
  - Cannot be undone - all job data will be lost
  - Daemon will restart automatically on next gob command

Exit codes:
  0: Nuke completed successfully
  1: Error (failed to stop jobs, failed to shutdown)`,
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

		// Nuke all jobs (empty workdir = no filter)
		stopped, logsDeleted, cleaned, err := client.Nuke("")
		if err != nil {
			return fmt.Errorf("failed to nuke: %w", err)
		}

		// Print summary
		fmt.Printf("Stopped %d running job(s)\n", stopped)
		fmt.Printf("Deleted %d log file(s)\n", logsDeleted)
		fmt.Printf("Cleaned up %d total job(s)\n", cleaned)

		// Shutdown daemon
		if err := client.Shutdown(); err != nil {
			return fmt.Errorf("failed to shutdown daemon: %w", err)
		}
		fmt.Println("Daemon shut down")

		return nil
	},
}

func init() {
	RootCmd.AddCommand(nukeCmd)
}
