package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/spf13/cobra"
)

var (
	portsAll  bool
	portsJSON bool
)

var portsCmd = &cobra.Command{
	Use:               "ports [job_id]",
	Short:             "List listening ports for jobs",
	ValidArgsFunction: completeJobIDs,
	Long: `List listening ports for jobs.

If a job_id is provided, shows ports for that specific job.
Otherwise, shows ports for all running jobs in the current directory.
Use --all to see ports from all directories.

This includes ports opened by child processes spawned by the job.

Output format (single job):
  PORT   PROTO  ADDRESS      PID
  8080   tcp    0.0.0.0      1234
  8081   tcp    127.0.0.1    1235

Output format (multiple jobs):
  JOB    PORT   PROTO  ADDRESS      PID
  abc    8080   tcp    0.0.0.0      1234
  def    3000   tcp    0.0.0.0      5678

Exit codes:
  0: Success
  1: Error (job not found)`,
	Args: cobra.MaximumNArgs(1),
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

		if len(args) == 1 {
			// Get ports for specific job
			jobID := args[0]
			return showJobPorts(cmd, client, jobID)
		}

		// Get ports for all running jobs
		return showAllPorts(cmd, client)
	},
}

func showJobPorts(cmd *cobra.Command, client *daemon.Client, jobID string) error {
	ports, err := client.Ports(jobID)
	if err != nil {
		return err
	}

	// Check if job is stopped
	if ports.Status == "stopped" {
		fmt.Printf("Job %s is not running\n", jobID)
		return nil
	}

	// Output as JSON or human-readable
	if portsJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(ports)
	}

	// Print ports
	if len(ports.Ports) == 0 {
		fmt.Printf("No listening ports for job %s\n", jobID)
		return nil
	}

	fmt.Printf("PORT   PROTO  ADDRESS      PID\n")
	for _, p := range ports.Ports {
		fmt.Printf("%-6d %-6s %-12s %d\n", p.Port, p.Protocol, p.Address, p.PID)
	}

	return nil
}

func showAllPorts(cmd *cobra.Command, client *daemon.Client) error {
	// Determine workdir filter
	var workdirFilter string
	if !portsAll {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}
		workdirFilter = cwd
	}

	allPorts, err := client.AllPorts(workdirFilter)
	if err != nil {
		return fmt.Errorf("failed to get ports: %w", err)
	}

	// Output as JSON or human-readable
	if portsJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(allPorts)
	}

	// Check if any ports
	totalPorts := 0
	for _, jp := range allPorts {
		totalPorts += len(jp.Ports)
	}

	if totalPorts == 0 {
		fmt.Println("No listening ports")
		return nil
	}

	// Print ports
	fmt.Printf("JOB    PORT   PROTO  ADDRESS      PID\n")
	for _, jp := range allPorts {
		for _, p := range jp.Ports {
			fmt.Printf("%-6s %-6d %-6s %-12s %d\n", jp.JobID, p.Port, p.Protocol, p.Address, p.PID)
		}
	}

	return nil
}

func init() {
	RootCmd.AddCommand(portsCmd)
	portsCmd.Flags().BoolVarP(&portsAll, "all", "a", false,
		"Show ports from all directories")
	portsCmd.Flags().BoolVar(&portsJSON, "json", false,
		"Output in JSON format")
}
