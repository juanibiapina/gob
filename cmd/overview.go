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

WORKFLOW
  gob start <command>     Start a background job
  gob list                List jobs with IDs and status
  gob stop <job_id>       Stop a job (--force for SIGKILL)
  gob restart <job_id>    Restart a job
  gob cleanup             Remove stopped job metadata

OUTPUT
  gob logs [job_id]       Follow stdout+stderr with prefixes
  gob stdout <job_id>     View raw stdout
  gob stderr <job_id>     View raw stderr

OTHER
  gob signal <job_id> <signal>   Send signal to job
  gob remove <job_id>            Remove single job metadata
  gob nuke                       Stop all + remove all

Job IDs are shown by 'gob list'.
Use 'gob <command> --help' for details.`)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(overviewCmd)
}
