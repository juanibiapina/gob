package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var awaitAnyTimeout int

var awaitAnyCmd = &cobra.Command{
	Use:   "await-any",
	Short: "Wait for any job to complete",
	Long: `Wait for any running job to complete.

Watches all running jobs in the current directory and exits when the first one finishes.
Shows a summary of the completed job and lists remaining jobs.

Examples:
  # Wait for any job to finish
  gob await-any

  # Wait with a timeout of 30 seconds
  gob await-any --timeout 30

Exit codes:
  Returns the exit code of the first job that completes.
  Returns 1 if there's an error (no jobs, connection failed).
  Returns 124 if timeout is reached (like GNU timeout).`,
	Args: cobra.NoArgs,
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

		// Get current directory for filtering
		workdirFilter, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Get list of jobs
		jobs, err := client.List(workdirFilter)
		if err != nil {
			return fmt.Errorf("failed to list jobs: %w", err)
		}

		// Filter to only running jobs
		var runningJobs []daemon.JobResponse
		for _, job := range jobs {
			if job.Status == "running" {
				runningJobs = append(runningJobs, job)
			}
		}

		if len(runningJobs) == 0 {
			fmt.Println("No running jobs to await")
			return nil
		}

		// Show what we're awaiting
		fmt.Printf("Awaiting %d job", len(runningJobs))
		if len(runningJobs) != 1 {
			fmt.Print("s")
		}
		fmt.Println("...")
		for _, job := range runningJobs {
			commandStr := strings.Join(job.Command, " ")
			fmt.Printf("  %s: %s\n", job.ID, commandStr)
		}
		fmt.Println()

		// Set up signal handling - on Ctrl+C, just exit (jobs continue in background)
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		// Create a fresh client for subscription (Subscribe uses the connection)
		subClient, err := daemon.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create subscription client: %w", err)
		}
		defer subClient.Close()

		if err := subClient.Connect(); err != nil {
			return fmt.Errorf("failed to connect subscription client: %w", err)
		}

		// Subscribe to events
		eventCh, errCh := subClient.SubscribeChan(workdirFilter)

		// Set up timeout if specified
		var timeoutCh <-chan time.Time
		if awaitAnyTimeout > 0 {
			timeoutCh = time.After(time.Duration(awaitAnyTimeout) * time.Second)
		}

		// Build set of job IDs we're watching
		watchingIDs := make(map[string]bool)
		for _, job := range runningJobs {
			watchingIDs[job.ID] = true
		}

		// Wait for first job to stop
		for {
			select {
			case <-sigCh:
				fmt.Println("\nInterrupted - jobs continue running in background")
				return nil

			case <-timeoutCh:
				fmt.Println("Timeout reached")
				os.Exit(124)

			case err := <-errCh:
				if err != nil {
					return fmt.Errorf("subscription error: %w", err)
				}
				return nil

			case event := <-eventCh:
				if event.Type == daemon.EventTypeJobStopped && watchingIDs[event.JobID] {
					// This is one of our jobs - show completion
					printJobSummary(&event.Job)

					// Show remaining jobs
					printRemainingJobs(runningJobs, event.JobID)

					// Exit with job's exit code
					if event.Job.ExitCode != nil && *event.Job.ExitCode != 0 {
						os.Exit(*event.Job.ExitCode)
					}
					return nil
				}
			}
		}
	},
}

// printRemainingJobs shows the jobs that are still running
func printRemainingJobs(jobs []daemon.JobResponse, excludeID string) {
	var remaining []daemon.JobResponse
	for _, job := range jobs {
		if job.ID != excludeID {
			remaining = append(remaining, job)
		}
	}

	if len(remaining) == 0 {
		return
	}

	fmt.Printf("\nRemaining job")
	if len(remaining) != 1 {
		fmt.Print("s")
	}
	fmt.Printf(" (%d):\n", len(remaining))

	for _, job := range remaining {
		commandStr := strings.Join(job.Command, " ")
		fmt.Printf("  %s: [%d] running: %s\n", job.ID, job.PID, commandStr)
	}
}

func init() {
	rootCmd.AddCommand(awaitAnyCmd)
	awaitAnyCmd.Flags().IntVarP(&awaitAnyTimeout, "timeout", "t", 0,
		"Timeout in seconds (exit 124 if reached)")
}
