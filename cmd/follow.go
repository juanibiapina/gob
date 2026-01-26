package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/juanibiapina/gob/internal/process"
	"github.com/juanibiapina/gob/internal/tail"
)

// DefaultStuckTimeoutMs is the timeout when no historical data (5 minutes)
const DefaultStuckTimeoutMs int64 = 5 * 60 * 1000

// NoOutputWindowMs is the constant "no output" window (1 minute)
const NoOutputWindowMs int64 = 60 * 1000

// FollowResult represents the result of following a job
type FollowResult struct {
	Completed     bool // job finished running
	PossiblyStuck bool // job may be stuck (timed out without output)
}

// followJob follows a job's output until it completes, is interrupted, or is detected as possibly stuck
// avgDurationMs is the average duration of successful runs (0 if no history)
// stdoutPath is the full path to the stdout log file
func followJob(jobID string, pid int, stdoutPath string, avgDurationMs int64) (FollowResult, error) {
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
		return FollowResult{}, fmt.Errorf("stdout log file not found: %s", stdoutPath)
	}
	if _, err := os.Stat(stderrPath); os.IsNotExist(err) {
		return FollowResult{}, fmt.Errorf("stderr log file not found: %s", stderrPath)
	}

	// Calculate stuck detection threshold
	// No data: 5 minutes
	// Has data: avg + 1 minute
	// Trigger: elapsed > threshold AND no output for 1 minute
	var stuckTimeoutMs int64
	if avgDurationMs == 0 {
		stuckTimeoutMs = DefaultStuckTimeoutMs
	} else {
		stuckTimeoutMs = avgDurationMs + NoOutputWindowMs
	}

	stuckTimeout := time.Duration(stuckTimeoutMs) * time.Millisecond
	noOutputWindow := time.Duration(NoOutputWindowMs) * time.Millisecond

	// Create follower
	follower := tail.NewFollower(os.Stdout)

	// Yellow ANSI color for stderr prefix (uses terminal theme)
	stderrPrefix := fmt.Sprintf("\033[33m[%s]\033[0m ", jobID)
	stdoutPrefix := fmt.Sprintf("[%s] ", jobID)

	follower.AddSource(tail.FileSource{Path: stdoutPath, Prefix: stdoutPrefix})
	follower.AddSource(tail.FileSource{Path: stderrPath, Prefix: stderrPrefix})

	// Set up signal handling - on Ctrl+C, just exit (job continues in background)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Track completion status
	result := FollowResult{}
	startTime := time.Now()

	// Monitor for process completion, signal, or stuck condition
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				// Check if process completed
				if !process.IsProcessRunning(pid) {
					// Give a moment for any final output to be written
					time.Sleep(200 * time.Millisecond)
					result.Completed = true
					follower.Stop()
					return
				}

				// Check for stuck condition
				// Trigger: elapsed > timeout AND no output for 1 minute
				elapsed := time.Since(startTime)
				timeSinceOutput := time.Since(follower.LastOutputTime())

				if elapsed > stuckTimeout && timeSinceOutput > noOutputWindow {
					result.PossiblyStuck = true
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

	return result, nil
}

// CalculateStuckTimeout returns the stuck detection timeout based on average duration
// No data: 5 minutes
// Has data: avg + 1 minute
func CalculateStuckTimeout(avgDurationMs int64) time.Duration {
	if avgDurationMs == 0 {
		return time.Duration(DefaultStuckTimeoutMs) * time.Millisecond
	}
	return time.Duration(avgDurationMs+NoOutputWindowMs) * time.Millisecond
}
