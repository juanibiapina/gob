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

func TestErrVersionMismatch_Error(t *testing.T) {
	err := &ErrVersionMismatch{
		DaemonVersion: "1.2.3",
		ClientVersion: "1.0.0",
	}

	msg := err.Error()

	// Should contain both versions
	if !strings.Contains(msg, "1.2.3") {
		t.Errorf("error message should contain daemon version, got: %s", msg)
	}
	if !strings.Contains(msg, "1.0.0") {
		t.Errorf("error message should contain client version, got: %s", msg)
	}

	// Should mention shutdown command
	if !strings.Contains(msg, "shutdown") {
		t.Errorf("error message should mention shutdown command, got: %s", msg)
	}
}

func TestErrVersionMismatch_ErrorsAs(t *testing.T) {
	err := &ErrVersionMismatch{
		DaemonVersion: "2.0.0",
		ClientVersion: "1.0.0",
	}

	var versionErr *ErrVersionMismatch
	if !errors.As(err, &versionErr) {
		t.Error("errors.As should match ErrVersionMismatch")
	}

	if versionErr.DaemonVersion != "2.0.0" {
		t.Errorf("expected daemon version 2.0.0, got %s", versionErr.DaemonVersion)
	}
	if versionErr.ClientVersion != "1.0.0" {
		t.Errorf("expected client version 1.0.0, got %s", versionErr.ClientVersion)
	}
}

func TestErrVersionMismatch_WrappedError(t *testing.T) {
	// Test that the error can be wrapped and still detected
	originalErr := &ErrVersionMismatch{
		DaemonVersion: "1.5.0",
		ClientVersion: "1.4.0",
	}

	wrappedErr := errors.New("connection failed: " + originalErr.Error())

	// Direct errors.As won't work on string wrapping, but our code uses it directly
	// This test ensures the error message is informative when wrapped
	if !strings.Contains(wrappedErr.Error(), "1.5.0") {
		t.Error("wrapped error should contain daemon version")
	}
}

func TestErrVersionMismatch_PreVersionNegotiation(t *testing.T) {
	// Test the special case for old daemons that don't support version negotiation
	err := &ErrVersionMismatch{
		DaemonVersion: "(pre-version-negotiation)",
		ClientVersion: "1.0.0",
	}

	msg := err.Error()
	if !strings.Contains(msg, "pre-version-negotiation") {
		t.Errorf("error message should indicate pre-version-negotiation daemon, got: %s", msg)
	}
}
