package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:                "add <command> [args...]",
	Short:              "Create and start a new background job",
	DisableFlagParsing: true,
	Long: `Create and start a new background job that continues running after the CLI exits.

The job is started as a detached process and assigned a unique job ID.
Use this ID with other commands to manage the job.

Examples:
  # Add a long-running sleep
  gob add sleep 3600

  # Add a server
  gob add python -m http.server 8080

  # Add a background compilation
  gob add make build

  # Quoted command strings also work
  gob add "make test"

  # Optional -- separator is also supported
  gob add -- npm run --flag

Output:
  Added job <job_id> running: <command>

Exit codes:
  0: Job added successfully
  1: Error (missing command, failed to start)`,
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

		return nil
	},
}

func init() {
	RootCmd.AddCommand(addCmd)
}
