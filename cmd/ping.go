package cmd

import (
	"fmt"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var pingCmd = &cobra.Command{
	Use:   "ping",
	Short: "Ping the daemon to verify it's running",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Create client
		client, err := daemon.NewClient()
		if err != nil {
			return err
		}
		defer client.Close()

		// Connect (auto-starts daemon if needed)
		if err := client.Connect(); err != nil {
			return fmt.Errorf("failed to connect to daemon: %w", err)
		}

		// Send ping
		if err := client.Ping(); err != nil {
			return fmt.Errorf("ping failed: %w", err)
		}

		fmt.Println("pong")
		return nil
	},
}

func init() {
	RootCmd.AddCommand(pingCmd)
}
