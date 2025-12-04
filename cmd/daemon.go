package cmd

import (
	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:    "daemon",
	Hidden: true, // Hidden from help - only used internally for auto-start
	Short:  "Run the gob daemon (internal use only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := daemon.New()
		if err != nil {
			return err
		}

		return d.Run()
	},
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}
