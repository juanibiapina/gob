package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var overviewCmd = &cobra.Command{
	Use:   "overview",
	Short: "Show overview and common usage patterns",
	Long:  `Display an overview of job management and common workflow patterns.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(`gob - Process Manager for AI agents (and humans)

RUNNING COMMANDS
  gob add <command>       Start a background job

WAITING
  gob await <job_id>      Wait for completion, stream output
  gob await-any           Wait for any job to complete
  gob await-all           Wait for all jobs to complete

PROCESS CONTROL
  gob start <job_id>      Start a stopped job
  gob stop <job_id>       Stop a job (--force for SIGKILL)
  gob restart <job_id>    Restart a job (stop + start)
  gob signal <job_id> <sig>  Send signal to job

OUTPUT
  gob logs                Follow stdout+stderr for all jobs
  gob stdout <job_id>     View stdout (--follow for real-time)
  gob stderr <job_id>     View stderr (--follow for real-time)

CLEANUP
  gob remove <job_id>     Remove a stopped job
  gob nuke                Stop all, remove all, shutdown daemon

INTERACTIVE
  gob tui                 Launch interactive TUI
  gob list                List jobs with IDs and status

Job IDs are 3 characters (e.g. abc, x7f).
Use 'gob <command> --help' for details.`)
		return nil
	},
}

func init() {
	RootCmd.AddCommand(overviewCmd)
}
