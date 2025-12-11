package cmd

import (
	"fmt"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var shutdownCmd = &cobra.Command{
	Use:   "shutdown",
	Short: "Stop all running jobs and shutdown daemon",
	Long: `Stop all running jobs and shutdown the daemon.

Workflow:
  1. Sends SIGTERM to all running jobs
  2. Waits for graceful termination (SIGKILL after timeout)
  3. Shuts down the daemon

Example:
  gob shutdown

Output:
  Stopped <n> running job(s)
  Daemon shut down

Notes:
  - Uses SIGTERM (graceful) not SIGKILL initially
  - Job history is preserved (jobs are not removed)
  - Daemon will restart automatically on next gob command

Exit codes:
  0: Shutdown completed successfully
  1: Error (failed to stop jobs, failed to shutdown)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Connect to daemon (skip version check - shutdown must always work)
		client, err := daemon.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		defer client.Close()

		if err := client.ConnectSkipVersionCheck(); err != nil {
			return fmt.Errorf("failed to connect to daemon: %w", err)
		}

		// Stop all running jobs
		stopped, err := client.StopAll()
		if err != nil {
			return fmt.Errorf("failed to stop jobs: %w", err)
		}

		// Print summary
		fmt.Printf("Stopped %d running job(s)\n", stopped)

		// Shutdown daemon
		if err := client.Shutdown(); err != nil {
			return fmt.Errorf("failed to shutdown daemon: %w", err)
		}
		fmt.Println("Daemon shut down")

		return nil
	},
}

func init() {
	RootCmd.AddCommand(shutdownCmd)
}
