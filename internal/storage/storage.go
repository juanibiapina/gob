package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/adrg/xdg"
)

// JobMetadata represents the metadata stored for each background job
type JobMetadata struct {
	ID      string   `json:"id"`
	Command []string `json:"command"`
	PID     int      `json:"pid"`
	Workdir string   `json:"workdir"` // Working directory where job was started
}

// JobInfo combines job ID with its metadata
type JobInfo struct {
	ID       string
	Metadata *JobMetadata
}

// GetJobDir returns the path to the centralized gob data directory using XDG
func GetJobDir() (string, error) {
	jobDir := filepath.Join(xdg.DataHome, "gob")
	return jobDir, nil
}

// GetCurrentWorkdir returns the current working directory
func GetCurrentWorkdir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current directory: %w", err)
	}
	return cwd, nil
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
func GenerateJobFilename(id string) string {
	return fmt.Sprintf("%s.json", id)
}

const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// GenerateJobID creates a unique job ID using base62-encoded millisecond timestamp
// This produces a 7-character sortable ID (e.g., "VkPZ0Yw")
func GenerateJobID() string {
	n := time.Now().UnixMilli()
	var result []byte
	for n > 0 {
		result = append([]byte{base62Chars[n%62]}, result...)
		n /= 62
	}
	return string(result)
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
	if metadata.ID == "" {
		metadata.ID = strings.TrimSuffix(filename, ".json")
	}

	return &metadata, nil
}

// ListJobMetadata reads job metadata files for the current working directory
// and returns them sorted by start time (newest first)
func ListJobMetadata() ([]JobInfo, error) {
	cwd, err := GetCurrentWorkdir()
	if err != nil {
		return nil, err
	}
	return listJobMetadataWithFilter(cwd)
}

// ListAllJobMetadata reads all job metadata files regardless of working directory
// and returns them sorted by start time (newest first)
func ListAllJobMetadata() ([]JobInfo, error) {
	return listJobMetadataWithFilter("")
}

// listJobMetadataWithFilter is the internal implementation
// If workdirFilter is empty, returns all jobs
// If workdirFilter is set, returns only jobs matching that workdir
func listJobMetadataWithFilter(workdirFilter string) ([]JobInfo, error) {
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

		// Apply workdir filter if specified
		if workdirFilter != "" && metadata.Workdir != workdirFilter {
			continue
		}

		// Extract job ID (filename without .json extension)
		jobID := strings.TrimSuffix(entry.Name(), ".json")

		jobs = append(jobs, JobInfo{
			ID:       jobID,
			Metadata: metadata,
		})
	}

	// Sort by ID (base62 timestamp), newest first
	// Base62 IDs are lexicographically sortable
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].ID > jobs[j].ID
	})

	return jobs, nil
}

// FindJobByCommand finds a job in the current directory with matching command+args
// Returns nil if no matching job is found
func FindJobByCommand(command []string) (*JobMetadata, error) {
	jobs, err := ListJobMetadata()
	if err != nil {
		return nil, err
	}

	for _, job := range jobs {
		if commandsEqual(job.Metadata.Command, command) {
			return job.Metadata, nil
		}
	}

	return nil, nil
}

// commandsEqual compares two command slices for equality
func commandsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
