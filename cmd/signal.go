package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"github.com/juanibiapina/gob/internal/storage"
	"github.com/spf13/cobra"
)

// parseSignal converts a signal name or number to a syscall.Signal
func parseSignal(signalStr string) (syscall.Signal, error) {
	// Try to parse as number first
	if num, err := strconv.Atoi(signalStr); err == nil {
		return syscall.Signal(num), nil
	}

	// Parse as signal name
	// Remove "SIG" prefix if present
	signalStr = strings.ToUpper(signalStr)
	signalStr = strings.TrimPrefix(signalStr, "SIG")

	// Map common signal names to syscall constants
	signalMap := map[string]syscall.Signal{
		"HUP":  syscall.SIGHUP,
		"INT":  syscall.SIGINT,
		"QUIT": syscall.SIGQUIT,
		"KILL": syscall.SIGKILL,
		"TERM": syscall.SIGTERM,
		"USR1": syscall.SIGUSR1,
		"USR2": syscall.SIGUSR2,
		"STOP": syscall.SIGSTOP,
		"CONT": syscall.SIGCONT,
		"ALRM": syscall.SIGALRM,
		"PIPE": syscall.SIGPIPE,
		"CHLD": syscall.SIGCHLD,
		"ABRT": syscall.SIGABRT,
		"TRAP": syscall.SIGTRAP,
	}

	if sig, ok := signalMap[signalStr]; ok {
		return sig, nil
	}

	return 0, fmt.Errorf("invalid signal: %s", signalStr)
}

var signalCmd = &cobra.Command{
	Use:   "signal <job_id> <signal>",
	Short: "Send a signal to a background job",
	Long: `Send a specific signal to a background job.

More flexible than 'job stop' - useful for custom signals like HUP (reload)
or USR1/USR2 (application-defined handlers).

Signal format:
  Accepts both names and numbers:
  - Names: TERM, SIGTERM, HUP, SIGHUP, INT, SIGINT, KILL, SIGKILL, etc.
  - Numbers: 1 (HUP), 2 (INT), 9 (KILL), 15 (TERM), etc.

Examples:
  # Reload configuration (common for servers)
  gob signal1732348944 HUP

  # Interrupt a job
  gob signal1732348944 INT

  # Send custom signal by number
  gob signal1732348944 10

  # Forcefully kill
  gob signal1732348944 KILL

Output:
  Sent signal <signal> to job <job_id> (PID <pid>)

Common signals:
  HUP (1)   - Hangup (often used for reload)
  INT (2)   - Interrupt (Ctrl+C)
  TERM (15) - Terminate (graceful shutdown)
  KILL (9)  - Kill (forceful, cannot be caught)
  USR1 (10) - User-defined signal 1
  USR2 (12) - User-defined signal 2

Notes:
  - Sending a signal to a stopped job is not an error (idempotent)
  - Use 'job list' to find job IDs

Exit codes:
  0: Signal sent successfully (or job already stopped)
  1: Error (job not found, invalid signal, failed to send)`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		jobID := args[0]
		signalStr := args[1]

		// Parse the signal
		sig, err := parseSignal(signalStr)
		if err != nil {
			return err
		}

		// Load job metadata
		metadata, err := storage.LoadJobMetadata(jobID + ".json")
		if err != nil {
			return fmt.Errorf("job not found: %s", jobID)
		}

		// Send the signal to the process
		// Note: This is idempotent - sending to a stopped job returns nil
		err = syscall.Kill(metadata.PID, sig)
		if err != nil {
			// Ignore "no such process" error for idempotency
			if err != syscall.ESRCH {
				return fmt.Errorf("failed to send signal to job %s: %w", jobID, err)
			}
		}

		// Print confirmation message
		fmt.Printf("Sent signal %s to job %s (PID %d)\n", signalStr, jobID, metadata.PID)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(signalCmd)
}
