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
	Short:              "Run a command and follow its output until completion",
	DisableFlagParsing: true,
	Long: `Run a command as a background job and follow its output until completion.

Smart job reuse:
  - If a job with the same command+args exists and is stopped → restart + follow
  - If a job with the same command+args exists and is running → error
  - If no matching job exists → create new job + follow

The job runs in the background, so if you press Ctrl+C or kill the CLI,
the job continues running normally.

Examples:
  # Run a build and follow its output
  gob run make build

  # Run tests (will reuse existing job if found)
  gob run make test

  # Run a command with flags (no special handling needed)
  gob run ls -la
  gob run pnpm --filter web typecheck

  # Quoted command strings also work
  gob run "make test"

  # Optional -- separator is also supported
  gob run -- python -m http.server 8080

Output:
  Shows the job's stdout and stderr in real-time until the job completes.

Exit codes:
  0: Job completed
  1: Error (failed to start job, job already running)`,
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

		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
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

		// Run the job (finds existing or creates new)
		result, err := client.Run(args, cwd)
		if err != nil {
			return fmt.Errorf("failed to add job: %w", err)
		}

		job := result.Job
		commandStr := strings.Join(job.Command, " ")

		if result.Restarted {
			fmt.Printf("Restarted job %s running: %s\n", job.ID, commandStr)
		} else {
			fmt.Printf("Added job %s running: %s\n", job.ID, commandStr)
		}

		// Small delay to let process start
		time.Sleep(50 * time.Millisecond)

		// Follow the output
		completed, err := followJob(job.ID, job.PID, job.StdoutPath)
		if err != nil {
			return err
		}

		if completed {
			// Poll for exit code (daemon sets it asynchronously after process exits)
			var exitCode *int
			for i := 0; i < 20; i++ { // Wait up to 2 seconds
				finalJob, err := client.GetJob(job.ID)
				if err == nil && finalJob.ExitCode != nil {
					exitCode = finalJob.ExitCode
					break
				}
				time.Sleep(100 * time.Millisecond)
			}

			if exitCode != nil {
				fmt.Printf("\nJob %s completed with exit code %d\n", job.ID, *exitCode)
				if *exitCode != 0 {
					os.Exit(*exitCode)
				}
			} else {
				fmt.Printf("\nJob %s completed\n", job.ID)
			}
		} else {
			fmt.Printf("\nJob %s continues running in background\n", job.ID)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
