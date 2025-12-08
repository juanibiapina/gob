package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// version is set via ldflags during build
var version = "dev"

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "gob",
	Short: "Process manager for AI agents (and humans)",
	Long: `A CLI to manage background processes with a shared interface for you and your AI coding agent.

Start a dev server with Claude Code, check its logs yourself. Or vice-versa.
Everyone has the same view. No more copy-pasting logs through chat.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// When called without subcommands, show overview
		return overviewCmd.RunE(cmd, args)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Set version for --version flag
	RootCmd.Version = version

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gob.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// RootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
