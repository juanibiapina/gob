package daemon

import (
	"fmt"
	"testing"
)

func TestNewRequest(t *testing.T) {
	tests := []struct {
		name    string
		reqType RequestType
	}{
		{"ping", RequestTypePing},
		{"add", RequestTypeAdd},
		{"list", RequestTypeList},
		{"stop", RequestTypeStop},
		{"subscribe", RequestTypeSubscribe},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := NewRequest(tt.reqType)

			if req.Type != tt.reqType {
				t.Errorf("expected type %s, got %s", tt.reqType, req.Type)
			}
			if req.Payload == nil {
				t.Error("expected non-nil payload map")
			}
			if len(req.Payload) != 0 {
				t.Error("expected empty payload map")
			}
		})
	}
}

func TestNewSuccessResponse(t *testing.T) {
	resp := NewSuccessResponse()

	if !resp.Success {
		t.Error("expected Success to be true")
	}
	if resp.Error != "" {
		t.Errorf("expected empty error, got %s", resp.Error)
	}
	if resp.Data == nil {
		t.Error("expected non-nil Data map")
	}
	if len(resp.Data) != 0 {
		t.Error("expected empty Data map")
	}
}

func TestNewErrorResponse(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"simple error", fmt.Errorf("test error"), "test error"},
		{"wrapped error", fmt.Errorf("outer: %w", fmt.Errorf("inner")), "outer: inner"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewErrorResponse(tt.err)

			if resp.Success {
				t.Error("expected Success to be false")
			}
			if resp.Error != tt.expected {
				t.Errorf("expected error %q, got %q", tt.expected, resp.Error)
			}
		})
	}
}

func TestRequestEncodeDecode(t *testing.T) {
	tests := []struct {
		name    string
		reqType RequestType
		payload map[string]interface{}
	}{
		{
			name:    "ping request",
			reqType: RequestTypePing,
			payload: map[string]interface{}{},
		},
		{
			name:    "add request with command",
			reqType: RequestTypeAdd,
			payload: map[string]interface{}{
				"command": []string{"echo", "hello"},
				"workdir": "/tmp",
			},
		},
		{
			name:    "stop request with force",
			reqType: RequestTypeStop,
			payload: map[string]interface{}{
				"job_id": "abc123",
				"force":  true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := NewRequest(tt.reqType)
			for k, v := range tt.payload {
				req.Payload[k] = v
			}

			// Encode
			data, err := EncodeRequest(req)
			if err != nil {
				t.Fatalf("failed to encode request: %v", err)
			}

			// Decode
			decoded, err := DecodeRequest(data)
			if err != nil {
				t.Fatalf("failed to decode request: %v", err)
			}

			if decoded.Type != req.Type {
				t.Errorf("expected type %s, got %s", req.Type, decoded.Type)
			}
		})
	}
}

func TestResponseEncodeDecode(t *testing.T) {
	tests := []struct {
		name string
		resp *Response
	}{
		{
			name: "success response",
			resp: &Response{
				Success: true,
				Data: map[string]interface{}{
					"message": "pong",
				},
			},
		},
		{
			name: "error response",
			resp: &Response{
				Success: false,
				Error:   "something went wrong",
			},
		},
		{
			name: "success with job data",
			resp: &Response{
				Success: true,
				Data: map[string]interface{}{
					"job": map[string]interface{}{
						"id":     "abc123",
						"status": "running",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			data, err := EncodeResponse(tt.resp)
			if err != nil {
				t.Fatalf("failed to encode response: %v", err)
			}

			// Decode
			decoded, err := DecodeResponse(data)
			if err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if decoded.Success != tt.resp.Success {
				t.Errorf("expected success %v, got %v", tt.resp.Success, decoded.Success)
			}
			if decoded.Error != tt.resp.Error {
				t.Errorf("expected error %q, got %q", tt.resp.Error, decoded.Error)
			}
		})
	}
}

func TestDecodeRequest_InvalidJSON(t *testing.T) {
	_, err := DecodeRequest([]byte("not valid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestDecodeResponse_InvalidJSON(t *testing.T) {
	_, err := DecodeResponse([]byte("not valid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
