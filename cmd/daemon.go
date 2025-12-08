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
		// Initialize logging first
		logPath, err := daemon.GetLogPath()
		if err != nil {
			return err
		}
		if err := daemon.InitLogger(logPath); err != nil {
			return err
		}

		d, err := daemon.New()
		if err != nil {
			return err
		}

		return d.Run()
	},
}

func init() {
	RootCmd.AddCommand(daemonCmd)
}
