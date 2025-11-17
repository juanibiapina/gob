package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/juanibiapina/job/internal/process"
	"github.com/juanibiapina/job/internal/storage"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run <command> [args...]",
	Short: "Run a command as a background job",
	Long: `Run a command as a background job that continues running after the CLI exits.
The job metadata (command, PID, timestamp) is stored in .local/share/job/ for later reference.`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// First argument is the command, rest are arguments
		command := args[0]
		commandArgs := []string{}
		if len(args) > 1 {
			commandArgs = args[1:]
		}

		// Start the detached process
		pid, err := process.StartDetached(command, commandArgs)
		if err != nil {
			return fmt.Errorf("failed to start job: %w", err)
		}

		// Get current working directory for metadata
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}

		// Create job metadata
		metadata := &storage.JobMetadata{
			Command:   args,
			PID:       pid,
			StartedAt: time.Now().Unix(),
			WorkDir:   workDir,
		}

		// Save metadata
		filename, err := storage.SaveJobMetadata(metadata)
		if err != nil {
			return fmt.Errorf("failed to save job metadata: %w", err)
		}

		// Extract job ID from filename (timestamp without .json extension)
		jobID := strings.TrimSuffix(filename, ".json")

		// Print confirmation message
		commandStr := strings.Join(args, " ")
		fmt.Printf("Started job %s running: %s\n", jobID, commandStr)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}
