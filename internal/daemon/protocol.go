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
	RequestTypeStop      RequestType = "stop"
	RequestTypeStart     RequestType = "start"
	RequestTypeRestart   RequestType = "restart"
	RequestTypeRemove    RequestType = "remove"
	RequestTypeCleanup   RequestType = "cleanup"
	RequestTypeNuke      RequestType = "nuke"
	RequestTypeSignal    RequestType = "signal"
	RequestTypeGetJob    RequestType = "get_job"
	RequestTypeRun       RequestType = "run"
	RequestTypeSubscribe RequestType = "subscribe"
)

// EventType represents the type of event emitted by the daemon
type EventType string

const (
	EventTypeJobAdded   EventType = "job_added"
	EventTypeJobStarted EventType = "job_started"
	EventTypeJobStopped EventType = "job_stopped"
	EventTypeJobRemoved EventType = "job_removed"
)

// Event represents a job state change event
type Event struct {
	Type     EventType   `json:"type"`
	JobID    string      `json:"job_id"`
	Job      JobResponse `json:"job,omitempty"`
	JobCount int         `json:"job_count"`
}

// Request represents a client request to the daemon
type Request struct {
	Type    RequestType            `json:"type"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// Response represents a daemon response to a client request
type Response struct {
	Success bool                   `json:"success"`
	Error   string                 `json:"error,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// JobResponse represents a job in API responses
type JobResponse struct {
	ID         string   `json:"id"`
	PID        int      `json:"pid"`
	Status     string   `json:"status"`
	Command    []string `json:"command"`
	Workdir    string   `json:"workdir"`
	CreatedAt  string   `json:"created_at"`
	StdoutPath string   `json:"stdout_path"`
	StderrPath string   `json:"stderr_path"`
	ExitCode   *int     `json:"exit_code,omitempty"`
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
