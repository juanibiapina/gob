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
	Use:                "run [--description <desc>] [--] <command> [args...]",
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

  # Run with a description
  gob run --description "Build project" make build
  gob run -d "Run tests" -- npm test

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

		// Capture current environment
		env := os.Environ()

		// Add job via daemon
		result, err := client.Add(commandArgs, cwd, env, description)
		if err != nil {
			return fmt.Errorf("failed to add job: %w", err)
		}

		// Print confirmation message
		commandStr := strings.Join(commandArgs, " ")
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
