package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"github.com/juanibiapina/gob/internal/daemon"
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
	upperStr := strings.ToUpper(signalStr)
	normalizedStr := strings.TrimPrefix(upperStr, "SIG")

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

	if sig, ok := signalMap[normalizedStr]; ok {
		return sig, nil
	}

	return 0, fmt.Errorf("invalid signal: %s", signalStr)
}

var signalCmd = &cobra.Command{
	Use:               "signal <job_id> <signal>",
	Short:             "Send a signal to a background job",
	ValidArgsFunction: completeJobIDs,
	Long: `Send a specific signal to a background job.

More flexible than 'job stop' - useful for custom signals like HUP (reload)
or USR1/USR2 (application-defined handlers).

Signal format:
  Accepts both names and numbers:
  - Names: TERM, SIGTERM, HUP, SIGHUP, INT, SIGINT, KILL, SIGKILL, etc.
  - Numbers: 1 (HUP), 2 (INT), 9 (KILL), 15 (TERM), etc.

Examples:
  # Reload configuration (common for servers)
  gob signal V3x0QqI HUP

  # Interrupt a job
  gob signal V3x0QqI INT

  # Send custom signal by number
  gob signal V3x0QqI 10

  # Forcefully kill
  gob signal V3x0QqI KILL

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

		// Connect to daemon
		client, err := daemon.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		defer client.Close()

		if err := client.Connect(); err != nil {
			return fmt.Errorf("failed to connect to daemon: %w", err)
		}

		// Send signal via daemon
		pid, err := client.Signal(jobID, sig)
		if err != nil {
			return err
		}

		// Print confirmation message
		fmt.Printf("Sent signal %s to job %s (PID %d)\n", signalStr, jobID, pid)

		return nil
	},
}

func init() {
	RootCmd.AddCommand(signalCmd)
}
