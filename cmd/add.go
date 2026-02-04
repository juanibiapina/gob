package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/juanibiapina/gob/internal/tui"
	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:                "add [--description <desc>] [--] <command> [args...]",
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

  # Add with a description
  gob add --description "Dev server" npm run dev
  gob add -d "Build watcher" -- npm run build:watch

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

		// Parse --description / -d flag manually (before --)
		var description string
		var commandArgs []string
		for i := 0; i < len(args); i++ {
			arg := args[i]
			if arg == "--" {
				// Everything after -- is the command
				commandArgs = args[i+1:]
				break
			}
			if arg == "--description" || arg == "-d" {
				if i+1 >= len(args) {
					return fmt.Errorf("--description requires a value")
				}
				description = args[i+1]
				i++ // skip the value
				continue
			}
			if strings.HasPrefix(arg, "--description=") {
				description = strings.TrimPrefix(arg, "--description=")
				continue
			}
			if strings.HasPrefix(arg, "-d=") {
				description = strings.TrimPrefix(arg, "-d=")
				continue
			}
			// Not a flag we recognize, treat rest as command
			commandArgs = args[i:]
			break
		}

		if len(commandArgs) == 0 {
			return fmt.Errorf("requires at least 1 arg(s)")
		}

		// Handle quoted command string: "echo hello world" -> ["echo", "hello", "world"]
		if len(commandArgs) == 1 && strings.Contains(commandArgs[0], " ") {
			commandArgs = strings.Fields(commandArgs[0])
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

		// Check if command is blocked in gobfile
		if blockedJob := tui.FindBlockedJob(cwd, commandArgs); blockedJob != nil {
			if blockedJob.Description != "" {
				return fmt.Errorf("job is blocked: %s", blockedJob.Description)
			}
			return fmt.Errorf("job is blocked")
		}

		// Capture current environment
		env := os.Environ()

		// Add job via daemon (blocked=false since CLI doesn't set blocked status)
		result, err := client.Add(commandArgs, cwd, env, description, false)
		if err != nil {
			return fmt.Errorf("failed to add job: %w", err)
		}

		commandStr := strings.Join(commandArgs, " ")

		// Print message based on action
		if result.Action == "already_running" {
			// Job was already running - just report that
			startedAt, _ := time.Parse(time.RFC3339, result.Job.StartedAt)
			duration := formatDuration(time.Since(startedAt))
			fmt.Printf("Job %s already running (since %s ago)\n", result.Job.ID, duration)
			fmt.Printf("  gob await %s   # wait for completion with live output\n", result.Job.ID)
			fmt.Printf("  gob stop %s    # stop the job\n", result.Job.ID)
		} else {
			// Job was created or started
			fmt.Printf("Added job %s running: %s\n", result.Job.ID, commandStr)

			// Show stats if job has previous runs
			if result.Job.RunCount > 0 {
				fmt.Printf("  Previous runs: %d (%.0f%% success rate)\n",
					result.Job.RunCount, result.Job.SuccessRate)
				if result.Job.SuccessCount >= 3 {
					fmt.Printf("  Expected duration if success: ~%s\n",
						formatDuration(time.Duration(result.Job.AvgDurationMs)*time.Millisecond))
				}
				if result.Job.FailureCount >= 3 {
					fmt.Printf("  Expected duration if failure: ~%s\n",
						formatDuration(time.Duration(result.Job.FailureAvgDurationMs)*time.Millisecond))
				}
			}

			fmt.Printf("  gob await %s   # wait for completion with live output\n", result.Job.ID)
			fmt.Printf("  gob stop %s    # stop the job\n", result.Job.ID)
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(addCmd)
}
