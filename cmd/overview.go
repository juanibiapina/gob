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

BASIC WORKFLOW

1. Add a job (starts in background):
   $ gob add sleep 300
   Started job 1732348944 running: sleep 300

2. List all jobs (shows job IDs, PIDs, and status):
   $ gob list
   1732348944: [12345] running: sleep 300
   1732348000: [12344] stopped: python server.py

3. Stop a job (use job ID, not PID):
   $ gob stop 1732348944
   Stopped job 1732348944 (PID 12345)

4. Clean up stopped jobs:
   $ gob cleanup
   Cleaned up 1 stopped job(s)

COMMON PATTERNS

Start multiple jobs:
  $ gob add python -m http.server 8080
  $ gob add npm run watch
  $ gob add make build

Restart a job:
  $ gob restart 1732348944

Force kill a stubborn job:
  $ gob stop 1732348944 --force

Send custom signal (e.g., reload config):
  $ gob signal 1732348944 HUP

Complete reset (stop all + remove all):
  $ gob nuke

IMPORTANT NOTES

- All commands use job IDs (Unix timestamps), not PIDs
- Job IDs persist even when processes stop/restart
- Use 'gob list' to find job IDs
- Exit codes: 0 = success, 1 = error

AVAILABLE COMMANDS

  add       Start a command as a background job
  list      List all jobs with their status
  stop      Stop a job (SIGTERM or SIGKILL with --force)
  start     Start a stopped job
  restart   Restart a job (stop + start)
  remove    Remove metadata for a single stopped job
  cleanup   Remove metadata for all stopped jobs
  nuke      Stop all jobs and remove all metadata
  signal    Send a specific signal to a job
  stdout    Display stdout output for a job
  stderr    Display stderr output for a job

Use 'gob [command] --help' for detailed information about each command.
`)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(overviewCmd)
}
