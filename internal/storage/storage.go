package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// JobMetadata represents the metadata stored for each background job
type JobMetadata struct {
	Command   []string `json:"command"`
	PID       int      `json:"pid"`
	StartedAt int64    `json:"started_at"`
	WorkDir   string   `json:"work_dir"`
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

// GenerateJobFilename creates a timestamp-based filename for a job
func GenerateJobFilename() string {
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%d.json", timestamp)
}

// SaveJobMetadata writes job metadata to a JSON file
func SaveJobMetadata(metadata *JobMetadata) (string, error) {
	jobDir, err := EnsureJobDir()
	if err != nil {
		return "", err
	}

	filename := GenerateJobFilename()
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

	return &metadata, nil
}
