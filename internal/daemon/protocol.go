// Package daemon implements the gob daemon and client-server protocol.
//
// # Communication Protocol
//
// The daemon and clients communicate over a Unix domain socket using JSON-encoded messages.
// Each connection handles one request-response cycle, except for subscriptions which
// stream events over a long-lived connection.
//
// Socket location: $XDG_RUNTIME_DIR/gob/daemon.sock (see paths.go)
//
// # Message Format
//
// Request (client to daemon):
//
//	{"type": "<RequestType>", "payload": {...}}
//
// Response (daemon to client):
//
//	{"success": true, "data": {...}}
//	{"success": false, "error": "message"}
//
// See [RequestType] constants for available request types and [EventType] for subscription events.
package daemon

import (
	"encoding/json"
	"fmt"
)

// RequestType represents the type of request being made to the daemon
type RequestType string

const (
	RequestTypePing      RequestType = "ping"
	RequestTypeShutdown  RequestType = "shutdown"
	RequestTypeList      RequestType = "list"
	RequestTypeAdd       RequestType = "add"
	RequestTypeCreate    RequestType = "create" // Add job without starting
	RequestTypeStop      RequestType = "stop"
	RequestTypeStart     RequestType = "start"
	RequestTypeRestart   RequestType = "restart"
	RequestTypeRemove    RequestType = "remove"
	RequestTypeStopAll   RequestType = "stop_all"
	RequestTypeSignal    RequestType = "signal"
	RequestTypeGetJob    RequestType = "get_job"
	RequestTypeRuns      RequestType = "runs"
	RequestTypeStats     RequestType = "stats"
	RequestTypeSubscribe RequestType = "subscribe"
	RequestTypeVersion   RequestType = "version"
	RequestTypePorts     RequestType = "ports"
	RequestTypeRemoveRun RequestType = "remove_run"
)

// EventType represents the type of event emitted by the daemon
type EventType string

const (
	EventTypeJobAdded     EventType = "job_added"
	EventTypeJobStarted   EventType = "job_started"
	EventTypeJobStopped   EventType = "job_stopped"
	EventTypeJobRemoved   EventType = "job_removed"
	EventTypeJobUpdated   EventType = "job_updated"
	EventTypeRunStarted   EventType = "run_started"
	EventTypeRunStopped   EventType = "run_stopped"
	EventTypeRunRemoved   EventType = "run_removed"
	EventTypePortsUpdated EventType = "ports_updated"
)

// Event represents a job/run state change event
type Event struct {
	Type            EventType      `json:"type"`
	JobID           string         `json:"job_id"`
	Job             JobResponse    `json:"job"`
	Run             *RunResponse   `json:"run,omitempty"`
	Stats           *StatsResponse `json:"stats,omitempty"`
	Ports           []PortInfo     `json:"ports,omitempty"` // For EventTypePortsUpdated
	JobCount        int            `json:"job_count"`
	RunningJobCount int            `json:"running_job_count"`
}

// Request represents a client request to the daemon
type Request struct {
	Type    RequestType    `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

// Response represents a daemon response to a client request
type Response struct {
	Success bool           `json:"success"`
	Error   string         `json:"error,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

// JobResponse represents a job in API responses
type JobResponse struct {
	ID          string     `json:"id"`
	PID         int        `json:"pid"`
	Status      string     `json:"status"`
	Command     []string   `json:"command"`
	Workdir     string     `json:"workdir"`
	Description string     `json:"description,omitempty"`
	CreatedAt   string     `json:"created_at"`
	StartedAt   string     `json:"started_at"`
	StoppedAt   string     `json:"stopped_at,omitempty"`
	StdoutPath  string     `json:"stdout_path"`
	StderrPath  string     `json:"stderr_path"`
	ExitCode    *int       `json:"exit_code,omitempty"`
	Ports       []PortInfo `json:"ports,omitempty"` // Listening ports (only for running jobs)
}

// RunResponse represents a run in API responses
type RunResponse struct {
	ID         string `json:"id"`
	JobID      string `json:"job_id"`
	PID        int    `json:"pid"`
	Status     string `json:"status"`
	ExitCode   *int   `json:"exit_code,omitempty"`
	StdoutPath string `json:"stdout_path"`
	StderrPath string `json:"stderr_path"`
	StartedAt  string `json:"started_at"`
	StoppedAt  string `json:"stopped_at,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// StatsResponse represents job statistics in API responses
type StatsResponse struct {
	JobID                string   `json:"job_id"`
	Command              []string `json:"command"`
	RunCount             int      `json:"run_count"`
	SuccessCount         int      `json:"success_count"`
	FailureCount         int      `json:"failure_count"`
	SuccessRate          float64  `json:"success_rate"`
	AvgDurationMs        int64    `json:"avg_duration_ms"`         // Average of successful runs
	FailureAvgDurationMs int64    `json:"failure_avg_duration_ms"` // Average of failed runs
	MinDurationMs        int64    `json:"min_duration_ms"`
	MaxDurationMs        int64    `json:"max_duration_ms"`
}

// AddResponse represents the response from adding a job
type AddResponse struct {
	Job    JobResponse    `json:"job"`
	Stats  *StatsResponse `json:"stats,omitempty"` // nil if no previous runs
	Action string         `json:"action"`          // "created", "started", or "already_running"
}

// NewRequest creates a new request with the given type
func NewRequest(reqType RequestType) *Request {
	return &Request{
		Type:    reqType,
		Payload: make(map[string]interface{}),
	}
}

// NewSuccessResponse creates a successful response
func NewSuccessResponse() *Response {
	return &Response{
		Success: true,
		Data:    make(map[string]interface{}),
	}
}

// NewErrorResponse creates an error response
func NewErrorResponse(err error) *Response {
	return &Response{
		Success: false,
		Error:   err.Error(),
	}
}

// EncodeRequest encodes a request to JSON
func EncodeRequest(req *Request) ([]byte, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}
	return data, nil
}

// DecodeRequest decodes a request from JSON
func DecodeRequest(data []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("failed to decode request: %w", err)
	}
	return &req, nil
}

// EncodeResponse encodes a response to JSON
func EncodeResponse(resp *Response) ([]byte, error) {
	data, err := json.Marshal(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to encode response: %w", err)
	}
	return data, nil
}

// DecodeResponse decodes a response from JSON
func DecodeResponse(data []byte) (*Response, error) {
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &resp, nil
}
