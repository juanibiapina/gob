package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/juanibiapina/gob/internal/process"
	"github.com/juanibiapina/gob/internal/tail"
)

// followJob follows a job's output until it completes or is interrupted
// Returns true if job completed, false if interrupted
// stdoutPath is the full path to the stdout log file
func followJob(jobID string, pid int, stdoutPath string) (bool, error) {
	// Derive stderr path from stdout path
	stderrPath := strings.Replace(stdoutPath, ".stdout.log", ".stderr.log", 1)

	// Wait for log files to exist
	for i := 0; i < 50; i++ {
		_, errStdout := os.Stat(stdoutPath)
		_, errStderr := os.Stat(stderrPath)
		if errStdout == nil && errStderr == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Check if log files exist
	if _, err := os.Stat(stdoutPath); os.IsNotExist(err) {
		return false, fmt.Errorf("stdout log file not found: %s", stdoutPath)
	}
	if _, err := os.Stat(stderrPath); os.IsNotExist(err) {
		return false, fmt.Errorf("stderr log file not found: %s", stderrPath)
	}

	// Create follower
	follower := tail.NewFollower(os.Stdout)

	// Orange ANSI color for stderr prefix
	orangePrefix := fmt.Sprintf("\033[38;5;208m[%s]\033[0m ", jobID)
	stdoutPrefix := fmt.Sprintf("[%s] ", jobID)

	follower.AddSource(tail.FileSource{Path: stdoutPath, Prefix: stdoutPrefix})
	follower.AddSource(tail.FileSource{Path: stderrPath, Prefix: orangePrefix})

	// Set up signal handling - on Ctrl+C, just exit (job continues in background)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Track completion status
	completed := false

	// Monitor for process completion or signal
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				if !process.IsProcessRunning(pid) {
					// Give a moment for any final output to be written
					time.Sleep(200 * time.Millisecond)
					completed = true
					follower.Stop()
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	// Wait for follower to finish or signal
	go func() {
		<-sigCh
		close(done)
		follower.Stop()
	}()

	follower.Wait()

	return completed, nil
}

// followJobByDir follows a job's output using storageDir to build paths
// This is a convenience wrapper for commands that still use the old pattern
func followJobByDir(jobID string, pid int, storageDir string) (bool, error) {
	stdoutPath := filepath.Join(storageDir, fmt.Sprintf("%s.stdout.log", jobID))
	return followJob(jobID, pid, stdoutPath)
}
