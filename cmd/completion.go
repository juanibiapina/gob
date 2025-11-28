package cmd

import (
	"strings"

	"github.com/juanibiapina/gob/internal/storage"
	"github.com/spf13/cobra"
)

// completeJobIDs provides completion for job IDs
func completeJobIDs(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	// Only complete first argument for most commands
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	jobs, err := storage.ListJobMetadata()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var completions []string
	for _, job := range jobs {
		if strings.HasPrefix(job.ID, toComplete) {
			// Format: jobID\tcommand (tab-separated for description)
			commandStr := strings.Join(job.Metadata.Command, " ")
			completions = append(completions, job.ID+"\t"+commandStr)
		}
	}

	return completions, cobra.ShellCompDirectiveNoFileComp
}
