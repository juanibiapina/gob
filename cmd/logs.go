package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/juanibiapina/gob/internal/tail"
	"github.com/spf13/cobra"
)

var followLogs bool

var logsCmd = &cobra.Command{
	Use:   "logs [job_id]",
	Short: "Display stdout and stderr for jobs",
	Long: `Display both stdout and stderr output for background jobs.

Without arguments, shows output for all jobs in the current directory.
With a job ID, shows output for that specific job.

In dump mode (default), reads existing log content and exits.
In follow mode (-f), streams output in real-time.

DUMP MODE (default):
  stdout content is written to stdout, stderr content is written to stderr.
  This preserves stream separation for downstream processing.

  # Dump all jobs in current directory
  gob logs

  # Dump a specific job
  gob logs V3x0QqI

  # Redirect stderr to see only stdout
  gob logs V3x0QqI 2>/dev/null

FOLLOW MODE (-f):
  Each line is prefixed with a tag in square brackets:
  - [<job_id>] default color prefix for stdout lines
  - [<job_id>] yellow prefix for stderr lines
  - [gob] cyan prefix for system events (process started/stopped)

  # Follow all jobs in current directory
  gob logs -f

  # Follow a specific job
  gob logs -f V3x0QqI

Exit codes:
  0: Success
  1: Error (job not found, log files not available)`,
	ValidArgsFunction: completeJobIDs,
	Args:              cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if followLogs {
			return logsFollow(args)
		}
		return logsDump(args)
	},
}

func logsDump(args []string) error {
	if len(args) == 1 {
		return logsDumpJob(args[0])
	}
	return logsDumpAll()
}

func logsDumpJob(jobID string) error {
	client, err := daemon.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	job, err := client.GetJob(jobID)
	if err != nil {
		return err
	}

	return dumpJobLogs(job)
}

func logsDumpAll() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	client, err := daemon.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	jobs, err := client.List(cwd)
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	for _, job := range jobs {
		if err := dumpJobLogs(&job); err != nil {
			return err
		}
	}

	return nil
}

func dumpJobLogs(job *daemon.JobResponse) error {
	if _, err := os.Stat(job.StdoutPath); err == nil {
		content, err := os.ReadFile(job.StdoutPath)
		if err != nil {
			return fmt.Errorf("failed to read stdout log: %w", err)
		}
		os.Stdout.Write(content)
	}

	if _, err := os.Stat(job.StderrPath); err == nil {
		content, err := os.ReadFile(job.StderrPath)
		if err != nil {
			return fmt.Errorf("failed to read stderr log: %w", err)
		}
		os.Stderr.Write(content)
	}

	return nil
}

func logsFollow(args []string) error {
	if len(args) == 1 {
		return logsFollowJob(args[0])
	}
	return logsFollowAll()
}

func logsFollowJob(jobID string) error {
	client, err := daemon.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer client.Close()

	if err := client.Connect(); err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	job, err := client.GetJob(jobID)
	if err != nil {
		return err
	}

	if _, err := os.Stat(job.StdoutPath); os.IsNotExist(err) {
		return fmt.Errorf("stdout log file not found: %s", job.StdoutPath)
	}
	if _, err := os.Stat(job.StderrPath); os.IsNotExist(err) {
		return fmt.Errorf("stderr log file not found: %s", job.StderrPath)
	}

	stderrPrefix := fmt.Sprintf("\033[33m[%s]\033[0m ", job.ID)
	stdoutPrefix := fmt.Sprintf("[%s] ", job.ID)

	return tail.FollowMultiple([]tail.FileSource{
		{Path: job.StdoutPath, Prefix: stdoutPrefix},
		{Path: job.StderrPath, Prefix: stderrPrefix},
	}, os.Stdout)
}

func logsFollowAll() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	listClient, err := daemon.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer listClient.Close()

	if err := listClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	follower := tail.NewFollower(os.Stdout)
	var mu sync.Mutex
	type runningJob struct {
		pid     int
		command string
	}
	runningJobs := make(map[string]runningJob)

	addJobSources := func(job *daemon.JobResponse) error {
		stdoutPath := job.StdoutPath
		stderrPath := job.StderrPath

		if _, err := os.Stat(stdoutPath); os.IsNotExist(err) {
			return fmt.Errorf("stdout log file not found: %s", stdoutPath)
		}
		if _, err := os.Stat(stderrPath); os.IsNotExist(err) {
			return fmt.Errorf("stderr log file not found: %s", stderrPath)
		}

		stderrPrefix := fmt.Sprintf("\033[33m[%s]\033[0m ", job.ID)
		stdoutPrefix := fmt.Sprintf("[%s] ", job.ID)

		follower.AddSource(tail.FileSource{Path: stdoutPath, Prefix: stdoutPrefix})
		follower.AddSource(tail.FileSource{Path: stderrPath, Prefix: stderrPrefix})

		runningJobs[job.ID] = runningJob{pid: job.PID, command: strings.Join(job.Command, " ")}
		return nil
	}

	jobs, err := listClient.List(cwd)
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	mu.Lock()
	for _, job := range jobs {
		if err := addJobSources(&job); err != nil {
			mu.Unlock()
			return err
		}
	}
	hasInitialJobs := len(jobs) > 0
	mu.Unlock()

	eventClient, err := daemon.NewClient()
	if err != nil {
		return fmt.Errorf("failed to create event client: %w", err)
	}
	defer eventClient.Close()

	if err := eventClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect event client: %w", err)
	}

	eventCh, errCh := eventClient.SubscribeChan(cwd)

	go func() {
		for {
			select {
			case event, ok := <-eventCh:
				if !ok {
					return
				}
				mu.Lock()
				switch event.Type {
				case daemon.EventTypeJobAdded, daemon.EventTypeJobStarted:
					if event.Job.Status == "running" {
						if _, exists := runningJobs[event.JobID]; !exists {
							addJobSources(&event.Job)
							follower.SystemLog("process started: %s (pid:%d id:%s)",
								strings.Join(event.Job.Command, " "), event.Job.PID, event.JobID)
						}
					}
				case daemon.EventTypeJobStopped:
					if job, exists := runningJobs[event.JobID]; exists {
						if event.Job.ExitCode != nil {
							follower.SystemLog("process stopped: %s (pid:%d id:%s) exit: %d",
								job.command, job.pid, event.JobID, *event.Job.ExitCode)
						} else {
							follower.SystemLog("process stopped: %s (pid:%d id:%s)",
								job.command, job.pid, event.JobID)
						}
						delete(runningJobs, event.JobID)
						if len(runningJobs) == 0 {
							follower.SystemLog("all processes stopped")
						}
					}
				}
				mu.Unlock()
			case err, ok := <-errCh:
				if ok && err != nil {
					follower.SystemLog("event subscription error: %v", err)
				}
				return
			}
		}
	}()

	if !hasInitialJobs {
		follower.SystemLog("waiting for jobs...")
	}

	return follower.Wait()
}

func init() {
	RootCmd.AddCommand(logsCmd)
	logsCmd.Flags().BoolVarP(&followLogs, "follow", "f", false, "Follow log output in real-time")
}
