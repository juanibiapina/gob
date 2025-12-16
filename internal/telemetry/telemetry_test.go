package telemetry

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/posthog/posthog-go"
)

// TestLoggerDoesNotOutputToStderr verifies that the telemetry logger
// does not output anything to stderr when requests fail.
// This is critical because stderr output would corrupt the TUI display.
func TestLoggerDoesNotOutputToStderr(t *testing.T) {
	// Create a mock server that always returns 500 errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("simulated failure"))
	}))
	defer server.Close()

	// Capture stderr
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stderr = w

	// Also redirect slog to capture its output
	var slogBuf bytes.Buffer
	oldDefault := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&slogBuf, nil)))

	// Create PostHog client with our logger pointing to the failing server
	testClient, err := posthog.NewWithConfig("test-key", posthog.Config{
		Endpoint: server.URL,
		Logger:   logger{},
		// Set small batch size and interval to trigger flush quickly
		BatchSize: 1,
	})
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Send an event (this will fail because the server returns 500)
	testClient.Enqueue(posthog.Capture{
		DistinctId: "test-user",
		Event:      "test-event",
	})

	// Close the client which triggers flush and will encounter errors
	testClient.Close()

	// Restore stderr and slog
	w.Close()
	os.Stderr = oldStderr
	slog.SetDefault(oldDefault)

	// Read captured stderr
	var stderrBuf bytes.Buffer
	io.Copy(&stderrBuf, r)
	r.Close()

	// Check that neither stderr nor slog captured any output
	stderrOutput := stderrBuf.String()
	slogOutput := slogBuf.String()

	if stderrOutput != "" {
		t.Errorf("expected no stderr output, got: %q", stderrOutput)
	}

	if slogOutput != "" {
		t.Errorf("expected no slog output, got: %q", slogOutput)
	}
}

// TestLoggerImplementsInterface verifies that our logger implements posthog.Logger
func TestLoggerImplementsInterface(t *testing.T) {
	var _ posthog.Logger = logger{}
}

// TestLoggerMethodsDoNotPanic verifies that all logger methods can be called without panicking
func TestLoggerMethodsDoNotPanic(t *testing.T) {
	l := logger{}

	// These should all be no-ops and not panic
	l.Debugf("debug message: %s", "test")
	l.Logf("log message: %s", "test")
	l.Warnf("warn message: %s", "test")
	l.Errorf("error message: %s", "test")
}

// TestLoggerErrorfDoesNotOutput specifically tests that Errorf produces no output
func TestLoggerErrorfDoesNotOutput(t *testing.T) {
	// Capture slog output
	var buf bytes.Buffer
	oldDefault := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(oldDefault)

	l := logger{}
	l.Errorf("this error should not appear: %s", "test error")

	output := buf.String()
	if strings.Contains(output, "error") || strings.Contains(output, "test") {
		t.Errorf("expected no output from Errorf, got: %q", output)
	}
}
