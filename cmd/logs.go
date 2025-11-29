package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/juanibiapina/gob/internal/storage"
	"github.com/juanibiapina/gob/internal/tail"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:               "logs [job_id]",
	Short:             "Follow both stdout and stderr for jobs",
	ValidArgsFunction: completeJobIDs,
	Long: `Follow both stdout and stderr output for background jobs in real-time.

Without arguments, follows all jobs started in the current directory.
With a job ID, follows only that specific job.

Streams both stdout and stderr as they are written. Stderr lines are prefixed
with "[err] " to distinguish them from stdout.

Example:
  # Follow all jobs in current directory
  gob logs

  # Follow a specific job
  gob logs V3x0QqI

Output:
  stdout line 1
  [err] error message
  stdout line 2

Notes:
  - Only works for jobs that have log files (jobs started with logging enabled)
  - Streams output in real-time as it's written
  - Press Ctrl+C to stop following

Exit codes:
  0: Stopped by user (Ctrl+C)
  1: Error (job not found, log files not available)`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobDir, err := storage.GetJobDir()
		if err != nil {
			return fmt.Errorf("failed to get job directory: %w", err)
		}

		var jobIDs []string

		if len(args) == 0 {
			// No arguments: follow all jobs in current directory
			jobs, err := storage.ListJobMetadata()
			if err != nil {
				return fmt.Errorf("failed to list jobs: %w", err)
			}
			if len(jobs) == 0 {
				return fmt.Errorf("no jobs found in current directory")
			}
			for _, job := range jobs {
				jobIDs = append(jobIDs, job.ID)
			}
		} else {
			// Specific job ID provided
			jobID := args[0]
			_, err := storage.LoadJobMetadata(jobID + ".json")
			if err != nil {
				return fmt.Errorf("job not found: %s", jobID)
			}
			jobIDs = []string{jobID}
		}

		// Build sources for all jobs
		var sources []tail.FileSource
		for _, jobID := range jobIDs {
			stdoutPath := filepath.Join(jobDir, fmt.Sprintf("%s.stdout.log", jobID))
			stderrPath := filepath.Join(jobDir, fmt.Sprintf("%s.stderr.log", jobID))

			// Check if log files exist
			if _, err := os.Stat(stdoutPath); os.IsNotExist(err) {
				return fmt.Errorf("stdout log file not found: %s", stdoutPath)
			}
			if _, err := os.Stat(stderrPath); os.IsNotExist(err) {
				return fmt.Errorf("stderr log file not found: %s", stderrPath)
			}

			sources = append(sources,
				tail.FileSource{Path: stdoutPath, Prefix: ""},
				tail.FileSource{Path: stderrPath, Prefix: "[err] "},
			)
		}

		return tail.FollowMultiple(sources, os.Stdout)
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
}
