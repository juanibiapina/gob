package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// version is set via ldflags during build
var version = "dev"

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gob",
	Short: "Start and manage background jobs",
	Long: `A CLI application to start and manage background jobs.

You can use this tool to add jobs in the background, monitor their status,
and manage their lifecycle.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// When called without subcommands, show overview
		return overviewCmd.RunE(cmd, args)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Set version for --version flag
	rootCmd.Version = version

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gob.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	// rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
