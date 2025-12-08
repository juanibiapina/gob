package cmd

import (
	"fmt"
	"os"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var cleanupAll bool

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove stopped jobs",
	Long: `Remove all stopped jobs.

By default, only removes stopped jobs in the current directory.
Use --all to cleanup stopped jobs from all directories.

Leaves running jobs untouched.

Example:
  # Remove stopped jobs from current directory
  gob cleanup

  # Remove stopped jobs from all directories
  gob cleanup --all

Output:
  Cleaned up <n> stopped job(s)

Examples:
  Cleaned up 3 stopped job(s)

  Or if nothing to clean:
  Cleaned up 0 stopped job(s)

Notes:
  - Only removes jobs that are no longer running
  - Does NOT stop any running jobs
  - Safe to run at any time
  - For removing a single job, use 'job remove <job_id>'

Exit codes:
  0: Cleanup completed successfully
  1: Error reading jobs`,
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
		if !cleanupAll {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
			workdirFilter = cwd
		}

		// Cleanup via daemon
		count, err := client.Cleanup(workdirFilter)
		if err != nil {
			return fmt.Errorf("failed to cleanup: %w", err)
		}

		// Print summary
		fmt.Printf("Cleaned up %d stopped job(s)\n", count)

		return nil
	},
}

func init() {
	RootCmd.AddCommand(cleanupCmd)
	cleanupCmd.Flags().BoolVarP(&cleanupAll, "all", "a", false,
		"Cleanup stopped jobs from all directories")
}
