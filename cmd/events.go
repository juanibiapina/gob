package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var eventsAll bool

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Subscribe to daemon events",
	Long: `Subscribe to daemon events and print them as JSON.

By default, only shows events for jobs in the current directory.
Use --all to see events from all directories.

Events are printed as JSON objects, one per line:
  {"type":"job_added","job_id":"V3x0QqI","job":{...}}
  {"type":"job_stopped","job_id":"V3x0QqI","job":{...}}

Event types:
  job_added   - A new job was created
  job_started - A stopped job was started
  job_stopped - A running job was stopped
  job_removed - A job was removed

This is useful for testing and debugging event subscriptions.
Press Ctrl+C to stop.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Connect to daemon
		client, err := daemon.NewClient()
		if err != nil {
			return fmt.Errorf("failed to create client: %w", err)
		}
		defer client.Close()

		if err := client.Connect(); err != nil {
			return fmt.Errorf("failed to connect to daemon: %w", err)
		}

		// Determine workdir filter
		var workdir string
		if !eventsAll {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current directory: %w", err)
			}
			workdir = cwd
		}

		// Subscribe to events
		encoder := json.NewEncoder(cmd.OutOrStdout())
		err = client.Subscribe(workdir, func(event daemon.Event) error {
			return encoder.Encode(event)
		})

		if err != nil {
			return fmt.Errorf("subscription error: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(eventsCmd)
	eventsCmd.Flags().BoolVarP(&eventsAll, "all", "a", false,
		"Show events from all directories")
}
