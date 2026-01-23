package tui

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/juanibiapina/gob/internal/daemon"
)

const gobfilePath = ".config/gobfile"

// ReadGobfile reads .config/gobfile from the given directory.
// Returns nil, nil if file doesn't exist.
// Returns commands as slice of strings (one per line).
func ReadGobfile(cwd string) ([]string, error) {
	path := filepath.Join(cwd, gobfilePath)

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var commands []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			commands = append(commands, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return commands, nil
}

// StartGobfileJobs starts jobs for gobfile commands.
// - If job exists and is running → skip
// - If job exists and is stopped → start it
// - If job doesn't exist → add it
// Continues on error, logs failures.
func StartGobfileJobs(cwd string, commands []string, env []string) error {
	client, err := daemon.NewClient()
	if err != nil {
		log.Printf("gobfile: failed to create client: %v", err)
		return err
	}
	if err := client.Connect(); err != nil {
		log.Printf("gobfile: failed to connect: %v", err)
		return err
	}
	defer client.Close()

	// Get existing jobs for this workdir
	existingJobs, err := client.List(cwd)
	if err != nil {
		log.Printf("gobfile: failed to list jobs: %v", err)
		return err
	}

	// Build a map of command -> job
	existingByCommand := make(map[string]daemon.JobResponse)
	for _, job := range existingJobs {
		cmdStr := strings.Join(job.Command, " ")
		existingByCommand[cmdStr] = job
	}

	// Process each gobfile command
	for _, cmd := range commands {
		parts := strings.Fields(cmd)
		if len(parts) == 0 {
			continue
		}

		if job, exists := existingByCommand[cmd]; exists {
			// Job exists - start it if stopped, skip if running
			if job.Status == "running" {
				continue
			}
			_, err := client.Start(job.ID, env)
			if err != nil {
				log.Printf("gobfile: failed to start '%s': %v", cmd, err)
				// Continue on error
			}
		} else {
			// Job doesn't exist - add it
			_, err := client.Add(parts, cwd, env)
			if err != nil {
				log.Printf("gobfile: failed to add '%s': %v", cmd, err)
				// Continue on error
			}
		}
	}

	return nil
}

// StopGobfileJobs stops running jobs that match gobfile commands.
// Continues on error.
func StopGobfileJobs(cwd string, commands []string) error {
	client, err := daemon.NewClient()
	if err != nil {
		log.Printf("gobfile: failed to create client: %v", err)
		return err
	}
	if err := client.Connect(); err != nil {
		log.Printf("gobfile: failed to connect: %v", err)
		return err
	}
	defer client.Close()

	// Get existing jobs for this workdir
	existingJobs, err := client.List(cwd)
	if err != nil {
		log.Printf("gobfile: failed to list jobs: %v", err)
		return err
	}

	// Build a set of gobfile commands for quick lookup
	gobfileCommands := make(map[string]bool)
	for _, cmd := range commands {
		gobfileCommands[cmd] = true
	}

	// Stop running jobs that match gobfile commands
	for _, job := range existingJobs {
		if job.Status != "running" {
			continue
		}

		cmdStr := strings.Join(job.Command, " ")
		if !gobfileCommands[cmdStr] {
			continue
		}

		_, err := client.Stop(job.ID, false)
		if err != nil {
			log.Printf("gobfile: failed to stop '%s': %v", cmdStr, err)
			// Continue on error
		}
	}

	return nil
}
