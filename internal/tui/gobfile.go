package tui

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/juanibiapina/gob/internal/daemon"
	"github.com/pelletier/go-toml/v2"
)

const gobfilePath = ".config/gobfile.toml"

// GobfileConfig represents the parsed gobfile.toml configuration
type GobfileConfig struct {
	Jobs []GobfileJob `toml:"job"`
}

// GobfileJob represents a single job in the gobfile
type GobfileJob struct {
	Command     string `toml:"command"`
	Description string `toml:"description"`
	Autostart   *bool  `toml:"autostart"` // nil defaults to true
}

// ShouldAutostart returns whether the job should be auto-started (defaults to true)
func (j GobfileJob) ShouldAutostart() bool {
	if j.Autostart == nil {
		return true
	}
	return *j.Autostart
}

// ReadGobfile reads .config/gobfile.toml from the given directory.
// Returns nil, nil if file doesn't exist.
// Returns parsed gobfile configuration.
func ReadGobfile(cwd string) (*GobfileConfig, error) {
	path := filepath.Join(cwd, gobfilePath)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var config GobfileConfig
	if err := toml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// StartGobfileJobs starts jobs for gobfile commands.
// - If job exists and is running → skip
// - If job exists and is stopped → start it (if autostart=true)
// - If job doesn't exist → add it (with or without starting based on autostart)
// Continues on error, logs failures.
func StartGobfileJobs(cwd string, config *GobfileConfig, env []string) error {
	if config == nil || len(config.Jobs) == 0 {
		return nil
	}

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

	// Process each gobfile job
	for _, gobJob := range config.Jobs {
		cmd := gobJob.Command
		parts := strings.Fields(cmd)
		if len(parts) == 0 {
			continue
		}

		if job, exists := existingByCommand[cmd]; exists {
			// Job exists - start it if stopped and autostart=true, skip if running
			if job.Status == "running" {
				continue
			}
			if gobJob.ShouldAutostart() {
				_, err := client.Start(job.ID, env)
				if err != nil {
					log.Printf("gobfile: failed to start '%s': %v", cmd, err)
					// Continue on error
				}
			}
		} else {
			// Job doesn't exist - add it
			if gobJob.ShouldAutostart() {
				// Add and start the job
				_, err := client.Add(parts, cwd, env, gobJob.Description)
				if err != nil {
					log.Printf("gobfile: failed to add '%s': %v", cmd, err)
					// Continue on error
				}
			} else {
				// Add without starting (create only)
				_, err := client.Create(parts, cwd, gobJob.Description)
				if err != nil {
					log.Printf("gobfile: failed to create '%s': %v", cmd, err)
					// Continue on error
				}
			}
		}
	}

	return nil
}

// StopGobfileJobs stops running jobs that match gobfile commands.
// Continues on error.
func StopGobfileJobs(cwd string, config *GobfileConfig) error {
	if config == nil || len(config.Jobs) == 0 {
		return nil
	}

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
	for _, job := range config.Jobs {
		gobfileCommands[job.Command] = true
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
