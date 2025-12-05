package cmd

import (
	"os"
	"strings"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

// completeJobIDs provides completion for job IDs
func completeJobIDs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Only complete first argument for most commands
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Get current workdir for filtering
	workdir, err := os.Getwd()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	// Connect to daemon
	client, err := daemon.NewClient()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	defer client.Close()

	if err := client.Connect(); err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	// List jobs for current workdir
	jobs, err := client.List(workdir)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var completions []string
	for _, job := range jobs {
		if strings.HasPrefix(job.ID, toComplete) {
			// Format: jobID\tcommand (tab-separated for description)
			commandStr := strings.Join(job.Command, " ")
			completions = append(completions, job.ID+"\t"+commandStr)
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}
