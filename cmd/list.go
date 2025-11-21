package cmd

import (
	"fmt"
	"strings"

	"github.com/juanibiapina/gob/internal/process"
	"github.com/juanibiapina/gob/internal/storage"
	"github.com/spf13/cobra"
)

var (
	listAll     bool
	showWorkdir bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List background jobs",
	Long: `List background jobs with their current status.

By default, only shows jobs started in the current directory.
Use --all to see jobs from all directories.

Shows job ID, PID, status (running/stopped), and the original command.
Use --workdir to also display the working directory for each job.
Jobs are sorted by start time (newest first).

Output format:
  <job_id>: [<pid>] <status>: <command>

With --workdir:
  <job_id>: [<pid>] <status> (<workdir>): <command>

Where:
  job_id: Unique identifier (Unix timestamp) - use this for other commands
  pid:    Process ID
  status: Either 'running' or 'stopped'
  workdir: Directory where job was started (only with --workdir or --all)
  command: Original command that was executed

Example output:
  1732350000: [12345] running: sleep 3600
  1732349000: [12344] stopped: python server.py

Example with --workdir:
  1732350000: [12345] running (/home/user/project): sleep 3600

If no jobs exist:
  No jobs found

Exit codes:
  0: Success
  1: Error reading jobs`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var jobs []storage.JobInfo
		var err error

		// Get jobs based on --all flag
		if listAll {
			jobs, err = storage.ListAllJobMetadata()
			showWorkdir = true // Always show workdir with --all
		} else {
			jobs, err = storage.ListJobMetadata()
		}

		if err != nil {
			return fmt.Errorf("failed to list jobs: %w", err)
		}

		// If no jobs, print message
		if len(jobs) == 0 {
			fmt.Println("No jobs found")
			return nil
		}

		// Print each job
		for _, job := range jobs {
			// Check if process is running
			var status string
			if process.IsProcessRunning(job.Metadata.PID) {
				status = "running"
			} else {
				status = "stopped"
			}

			// Format command as a single string
			commandStr := strings.Join(job.Metadata.Command, " ")

			// Print with or without workdir
			if showWorkdir {
				workdir := job.Metadata.Workdir
				if workdir == "" {
					workdir = "<unknown>" // For legacy jobs without workdir
				}
				fmt.Printf("%s: [%d] %s (%s): %s\n",
					job.ID, job.Metadata.PID, status, workdir, commandStr)
			} else {
				fmt.Printf("%s: [%d] %s: %s\n",
					job.ID, job.Metadata.PID, status, commandStr)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVarP(&listAll, "all", "a", false,
		"Show jobs from all directories (implies --workdir)")
	listCmd.Flags().BoolVar(&showWorkdir, "workdir", false,
		"Show working directory for each job")
}
