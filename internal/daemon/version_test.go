package daemon

import (
	"errors"
	"strings"
	"testing"

	"github.com/juanibiapina/gob/internal/version"
)

// parseVersionResponse is a helper that extracts VersionInfo from a Response
// Used for testing the parsing logic without needing a real daemon connection
func parseVersionResponse(resp *Response) (*VersionInfo, error) {
	// Old daemon returns error for unknown request type
	if !resp.Success {
		if strings.Contains(resp.Error, "unknown request type") {
			return nil, ErrOldDaemon
		}
		return nil, errors.New(resp.Error)
	}

	// Parse version info from daemon
	return &VersionInfo{
		Version:     resp.Data["version"].(string),
		RunningJobs: int(resp.Data["running_jobs"].(float64)),
	}, nil
}

func TestVersionNegotiation_OldDaemon(t *testing.T) {
	// Simulate old daemon response
	resp := &Response{
		Success: false,
		Error:   "unknown request type: version",
	}

	// Client should detect this as old daemon
	_, err := parseVersionResponse(resp)
	if !errors.Is(err, ErrOldDaemon) {
		t.Errorf("expected ErrOldDaemon, got %v", err)
	}
}

func TestVersionNegotiation_NewDaemon(t *testing.T) {
	// Simulate new daemon response
	resp := &Response{
		Success: true,
		Data: map[string]interface{}{
			"version":      "1.2.3",
			"running_jobs": float64(0),
		},
	}

	info, err := parseVersionResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.Version != "1.2.3" {
		t.Errorf("expected version 1.2.3, got %s", info.Version)
	}
	if info.RunningJobs != 0 {
		t.Errorf("expected 0 running jobs, got %d", info.RunningJobs)
	}
}

func TestVersionNegotiation_NewDaemonWithRunningJobs(t *testing.T) {
	// Simulate new daemon response with running jobs
	resp := &Response{
		Success: true,
		Data: map[string]interface{}{
			"version":      "1.0.0",
			"running_jobs": float64(3),
		},
	}

	info, err := parseVersionResponse(resp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.RunningJobs != 3 {
		t.Errorf("expected 3 running jobs, got %d", info.RunningJobs)
	}
}

func TestVersionNegotiation_OtherError(t *testing.T) {
	// Simulate other error (not old daemon)
	resp := &Response{
		Success: false,
		Error:   "some other error",
	}

	_, err := parseVersionResponse(resp)
	if errors.Is(err, ErrOldDaemon) {
		t.Error("expected non-ErrOldDaemon error")
	}
	if err == nil {
		t.Error("expected an error")
	}
	if err.Error() != "some other error" {
		t.Errorf("expected 'some other error', got %v", err)
	}
}

func TestRequestTypeVersionConstant(t *testing.T) {
	// Version request type should be defined
	if RequestTypeVersion != "version" {
		t.Errorf("expected request type 'version', got %s", RequestTypeVersion)
	}
}

func TestVersionInfo_Struct(t *testing.T) {
	info := VersionInfo{
		Version:     version.Version,
		RunningJobs: 5,
	}

	if info.Version == "" {
		t.Error("expected non-empty version")
	}
	if info.RunningJobs != 5 {
		t.Errorf("expected 5 running jobs, got %d", info.RunningJobs)
	}
}
