package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/juanibiapina/gob/internal/process"
	"github.com/juanibiapina/gob/internal/storage"
	"github.com/spf13/cobra"
)

var (
	listAll     bool
	showWorkdir bool
	listJSON    bool
)

// JobOutput represents a job in JSON output format
type JobOutput struct {
	ID        string   `json:"id"`
	PID       int      `json:"pid"`
	Status    string   `json:"status"`
	Command   []string `json:"command"`
	Workdir   string   `json:"workdir"`
	CreatedAt string   `json:"created_at"`
}

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
  job_id: Unique identifier - use this for other commands
  pid:    Process ID
  status: Either 'running' or 'stopped'
  workdir: Directory where job was started (only with --workdir or --all)
  command: Original command that was executed

Example output:
  V3x0QqI: [12345] running: sleep 3600
  V3x0PrH: [12344] stopped: python server.py

Example with --workdir:
  V3x0QqI: [12345] running (/home/user/project): sleep 3600

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

		// If no jobs, print message (unless JSON output)
		if len(jobs) == 0 {
			if listJSON {
				fmt.Println("[]")
			} else {
				fmt.Println("No jobs found")
			}
			return nil
		}

		// Build job output list with status
		var jobOutputs []JobOutput
		for _, job := range jobs {
			var status string
			if process.IsProcessRunning(job.Metadata.PID) {
				status = "running"
			} else {
				status = "stopped"
			}

			jobOutputs = append(jobOutputs, JobOutput{
				ID:        job.ID,
				PID:       job.Metadata.PID,
				Status:    status,
				Command:   job.Metadata.Command,
				Workdir:   job.Metadata.Workdir,
				CreatedAt: job.Metadata.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			})
		}

		// Output as JSON or human-readable
		if listJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(jobOutputs)
		}

		// Print each job in human-readable format
		for _, job := range jobOutputs {
			commandStr := strings.Join(job.Command, " ")

			if showWorkdir {
				workdir := job.Workdir
				if workdir == "" {
					workdir = "<unknown>"
				}
				fmt.Printf("%s: [%d] %s (%s): %s\n",
					job.ID, job.PID, job.Status, workdir, commandStr)
			} else {
				fmt.Printf("%s: [%d] %s: %s\n",
					job.ID, job.PID, job.Status, commandStr)
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
	listCmd.Flags().BoolVar(&listJSON, "json", false,
		"Output in JSON format")
}
