package cmd

import (
	"github.com/juanibiapina/gob/internal/daemon"
	godaemon "github.com/sevlyar/go-daemon"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:    "daemon",
	Hidden: true, // Hidden from help - only used internally for auto-start
	Short:  "Run the gob daemon (internal use only)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Use go-daemon to properly daemonize with PPID=1
		// Don't use LogFileName - let InitLogger handle logging after daemonization
		ctx := &godaemon.Context{}

		child, err := ctx.Reborn()
		if err != nil {
			return err
		}
		if child != nil {
			// Parent process - exit immediately
			return nil
		}
		// Child process continues as daemon
		defer ctx.Release()

		// Initialize logging
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
