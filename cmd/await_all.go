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

var awaitAllTimeout int

var awaitAllCmd = &cobra.Command{
	Use:   "await-all",
	Short: "Wait for all jobs to complete",
	Long: `Wait for all running jobs to complete.

Watches all running jobs in the current directory and exits when all finish.
Shows a brief status for each job as it completes, then a final summary.

Examples:
  # Wait for all jobs to finish
  gob await-all

  # Wait with a timeout of 60 seconds
  gob await-all --timeout 60

Exit codes:
  Returns 0 if all jobs succeeded.
  Returns the first non-zero exit code if any job failed.
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
		if awaitAllTimeout > 0 {
			timeoutCh = time.After(time.Duration(awaitAllTimeout) * time.Second)
		}

		// Build set of job IDs we're watching
		pendingIDs := make(map[string]bool)
		for _, job := range runningJobs {
			pendingIDs[job.ID] = true
		}

		// Track completed jobs and first failure
		var completedJobs []daemon.JobResponse
		var firstFailureCode *int

		// Wait for all jobs to stop
		for len(pendingIDs) > 0 {
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
				if event.Type == daemon.EventTypeJobStopped && pendingIDs[event.JobID] {
					// This is one of our jobs - mark as completed
					delete(pendingIDs, event.JobID)
					completedJobs = append(completedJobs, event.Job)

					// Track first failure
					if firstFailureCode == nil && event.Job.ExitCode != nil && *event.Job.ExitCode != 0 {
						firstFailureCode = event.Job.ExitCode
					}

					// Show brief completion status
					printJobCompletionStatus(&event.Job, len(pendingIDs))
				}
			}
		}

		// Show final summary
		fmt.Println()
		printAllJobsSummary(completedJobs)

		// Exit with first failure code if any
		if firstFailureCode != nil {
			os.Exit(*firstFailureCode)
		}

		return nil
	},
}

// printJobCompletionStatus shows a brief one-line status when a job completes
func printJobCompletionStatus(job *daemon.JobResponse, remaining int) {
	commandStr := strings.Join(job.Command, " ")
	status := "✓"
	if job.ExitCode != nil && *job.ExitCode != 0 {
		status = fmt.Sprintf("✗ (%d)", *job.ExitCode)
	} else if job.ExitCode == nil {
		status = "◼"
	}

	remainingStr := ""
	if remaining > 0 {
		remainingStr = fmt.Sprintf(" [%d remaining]", remaining)
	}

	fmt.Printf("%s %s: %s%s\n", status, job.ID, commandStr, remainingStr)
}

// printAllJobsSummary shows a summary of all completed jobs
func printAllJobsSummary(jobs []daemon.JobResponse) {
	succeeded := 0
	failed := 0
	killed := 0

	for _, job := range jobs {
		if job.ExitCode == nil {
			killed++
		} else if *job.ExitCode == 0 {
			succeeded++
		} else {
			failed++
		}
	}

	fmt.Printf("All jobs completed: %d succeeded", succeeded)
	if failed > 0 {
		fmt.Printf(", %d failed", failed)
	}
	if killed > 0 {
		fmt.Printf(", %d killed", killed)
	}
	fmt.Println()
}

func init() {
	rootCmd.AddCommand(awaitAllCmd)
	awaitAllCmd.Flags().IntVarP(&awaitAllTimeout, "timeout", "t", 0,
		"Timeout in seconds (exit 124 if reached)")
}
