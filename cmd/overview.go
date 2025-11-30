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
		fmt.Println(`gob - Background Job Manager

JOB MANAGEMENT
  gob add <command>       Create and start a new job
  gob remove <job_id>     Remove job metadata
  gob cleanup             Remove all stopped jobs
  gob nuke                Stop all + remove all

PROCESS CONTROL
  gob start <job_id>      Start a stopped job
  gob stop <job_id>       Stop a job (--force for SIGKILL)
  gob restart <job_id>    Restart a job (stop + start)
  gob signal <job_id> <sig>  Send signal to job

CONVENIENCE
  gob run <command>       Run command (reuses existing stopped job)

OUTPUT
  gob logs                Follow stdout+stderr for all jobs
  gob stdout <job_id>     View raw stdout
  gob stderr <job_id>     View raw stderr

INTERACTIVE
  gob tui                 Launch interactive TUI

Use -f/--follow with add/start/restart to follow output.
Job IDs are shown by 'gob list'.
Use 'gob <command> --help' for details.`)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(overviewCmd)
}
