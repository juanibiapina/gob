package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var (
	listAll     bool
	showWorkdir bool
	listJSON    bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List background jobs",
	Long: `List background jobs with their current status.

By default, only shows jobs started in the current directory.
Use --all to see jobs from all directories.

Shows job ID, PID, status (running/stopped), and the original command.
If a job has a description, it is shown on a second indented line.
Use --workdir to also display the working directory for each job.
Jobs are sorted by start time (newest first).

Output format:
  <job_id>: [<pid>] <status>: <command>
           <description>   (if present)

With --workdir:
  <job_id>: [<pid>] <status> (<workdir>): <command>

Where:
  job_id: Unique identifier - use this for other commands
  pid:    Process ID (or "-" if stopped)
  status: Either 'running' or 'stopped'
  workdir: Directory where job was started (only with --workdir or --all)
  command: Original command that was executed

Example output:
  V3x0QqI: [12345] running: npm run dev
           Development server for the frontend app
  V3x0PrH: [-] stopped: npm run build:watch
           Watches TypeScript and rebuilds on change

Example with --workdir:
  V3x0QqI: [12345] running (/home/user/project): sleep 3600

If no jobs exist:
  No jobs found

Exit codes:
  0: Success
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
		if !listAll {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
			workdirFilter = cwd
		} else {
			showWorkdir = true // Always show workdir with --all
		}

		// Get jobs from daemon
		jobs, err := client.List(workdirFilter)
		if err != nil {
			return fmt.Errorf("failed to list jobs: %w", err)
		}

		// If no jobs, print message (unless JSON output)
		if len(jobs) == 0 {
			if listJSON {
				fmt.Println("[]")
			} else {
				fmt.Println("No jobs found")
			}
			return nil
		}

		// Output as JSON or human-readable
		if listJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(jobs)
		}

		// Print each job in human-readable format
		for _, job := range jobs {
			commandStr := strings.Join(job.Command, " ")

			// Format status with exit code if available
			status := job.Status
			if job.ExitCode != nil {
				status = fmt.Sprintf("%s (%d)", job.Status, *job.ExitCode)
			}

			// Format PID (show "-" for stopped jobs with no PID)
			pidStr := fmt.Sprintf("%d", job.PID)
			if job.PID == 0 {
				pidStr = "-"
			}

			if showWorkdir {
				workdir := job.Workdir
				if workdir == "" {
					workdir = "<unknown>"
				}
				fmt.Printf("%s: [%s] %s (%s): %s\n",
					job.ID, pidStr, status, workdir, commandStr)
			} else {
				fmt.Printf("%s: [%s] %s: %s\n",
					job.ID, pidStr, status, commandStr)
			}

			// Print description on second line if present
			if job.Description != "" {
				// Indent to align with command (9 spaces for "XXX: [-] ")
				fmt.Printf("         %s\n", job.Description)
			}
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(listCmd)
	listCmd.Flags().BoolVarP(&listAll, "all", "a", false,
		"Show jobs from all directories (implies --workdir)")
	listCmd.Flags().BoolVar(&showWorkdir, "workdir", false,
		"Show working directory for each job")
	listCmd.Flags().BoolVar(&listJSON, "json", false,
		"Output in JSON format")
}
