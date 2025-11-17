package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// JobMetadata represents the metadata stored for each background job
type JobMetadata struct {
	ID         int64    `json:"id"`
	Command    []string `json:"command"`
	PID        int      `json:"pid"`
	StdoutFile string   `json:"stdout_file,omitempty"`
	StderrFile string   `json:"stderr_file,omitempty"`
}

// JobInfo combines job ID with its metadata
type JobInfo struct {
	ID       string
	Metadata *JobMetadata
}

// GetJobDir returns the path to the .local/share/job directory in the current working directory
func GetJobDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}

	jobDir := filepath.Join(cwd, ".local", "share", "job")
	return jobDir, nil
}

// EnsureJobDir creates the job directory if it doesn't exist
func EnsureJobDir() (string, error) {
	jobDir, err := GetJobDir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create job directory: %w", err)
	}

	return jobDir, nil
}

// GenerateJobFilename creates a filename for a job based on its ID
func GenerateJobFilename(id int64) string {
	return fmt.Sprintf("%d.json", id)
}

// SaveJobMetadata writes job metadata to a JSON file
func SaveJobMetadata(metadata *JobMetadata) (string, error) {
	jobDir, err := EnsureJobDir()
	if err != nil {
		return "", err
	}

	filename := GenerateJobFilename(metadata.ID)
	filepath := filepath.Join(jobDir, filename)

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal job metadata: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write job metadata: %w", err)
	}

	return filename, nil
}

// LoadJobMetadata reads job metadata from a JSON file
func LoadJobMetadata(filename string) (*JobMetadata, error) {
	jobDir, err := GetJobDir()
	if err != nil {
		return nil, err
	}

	filepath := filepath.Join(jobDir, filename)
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read job metadata: %w", err)
	}

	var metadata JobMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal job metadata: %w", err)
	}

	// If ID is not in the metadata (backward compatibility), extract from filename
	if metadata.ID == 0 {
		jobID := strings.TrimSuffix(filename, ".json")
		// Parse the ID from filename (best effort)
		fmt.Sscanf(jobID, "%d", &metadata.ID)
	}

	return &metadata, nil
}

// ListJobMetadata reads all job metadata files and returns them sorted by start time (newest first)
func ListJobMetadata() ([]JobInfo, error) {
	jobDir, err := GetJobDir()
	if err != nil {
		return nil, err
	}

	// Check if job directory exists
	if _, err := os.Stat(jobDir); os.IsNotExist(err) {
		return []JobInfo{}, nil
	}

	// Read all files in the job directory
	entries, err := os.ReadDir(jobDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read job directory: %w", err)
	}

	var jobs []JobInfo

	// Parse each JSON file
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		metadata, err := LoadJobMetadata(entry.Name())
		if err != nil {
			// Skip files that can't be parsed
			continue
		}

		// Extract job ID (filename without .json extension)
		jobID := strings.TrimSuffix(entry.Name(), ".json")

		jobs = append(jobs, JobInfo{
			ID:       jobID,
			Metadata: metadata,
		})
	}

	// Sort by ID (timestamp), newest first
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].Metadata.ID > jobs[j].Metadata.ID
	})

	return jobs, nil
}
