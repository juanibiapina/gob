package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var addFollow bool

var addCmd = &cobra.Command{
	Use:   "add <command> [args...]",
	Short: "Create and start a new background job",
	Long: `Create and start a new background job that continues running after the CLI exits.

The job is started as a detached process and assigned a unique job ID.
Use this ID with other commands to manage the job.

Use -- to separate gob flags from the command when the command has flags:
  gob add -- python -m http.server 8080

Examples:
  # Add a long-running sleep
  gob add sleep 3600

  # Add a server (use -- before commands with flags)
  gob add -- python -m http.server 8080

  # Add a background compilation
  gob add make build

  # Add and follow output until completion
  gob add -f -- make test

Output:
  Added job <job_id> running: <command>

Exit codes:
  0: Job added successfully
  1: Error (missing command, failed to start)`,
	Args: cobra.MinimumNArgs(1),
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

		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Add job via daemon
		job, err := client.Add(args, cwd)
		if err != nil {
			return fmt.Errorf("failed to add job: %w", err)
		}

		// Print confirmation message
		commandStr := strings.Join(args, " ")
		fmt.Printf("Added job %s running: %s\n", job.ID, commandStr)
		fmt.Printf("  gob await %s   # wait for completion with live output\n", job.ID)
		fmt.Printf("  gob stop %s    # stop the job\n", job.ID)

		// If follow flag is set, follow the output
		if addFollow {
			completed, err := followJob(job.ID, job.PID, job.StdoutPath)
			if err != nil {
				return err
			}
			if completed {
				fmt.Printf("\nJob %s completed\n", job.ID)
			} else {
				fmt.Printf("\nJob %s continues running in background\n", job.ID)
			}
		}

		return nil
	},
}

func init() {
	addCmd.Flags().BoolVarP(&addFollow, "follow", "f", false, "Follow output until job completes")
	rootCmd.AddCommand(addCmd)
}
