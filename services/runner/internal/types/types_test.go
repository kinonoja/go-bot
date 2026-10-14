package types

import (
	"encoding/json"
	"testing"
)

func TestRunRequestJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantMode string
		wantLen  int
	}{
		{
			name:     "basic request",
			input:    `{"files":{"main.go":"package main"},"mode":"run","timeout_ms":5000}`,
			wantMode: "run",
			wantLen:  1,
		},
		{
			name:     "test mode",
			input:    `{"files":{"main.go":"a","main_test.go":"b"},"mode":"test","timeout_ms":3000}`,
			wantMode: "test",
			wantLen:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req RunRequest
			if err := json.Unmarshal([]byte(tt.input), &req); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if req.Mode != tt.wantMode {
				t.Errorf("mode = %q, want %q", req.Mode, tt.wantMode)
			}
			if len(req.Files) != tt.wantLen {
				t.Errorf("files len = %d, want %d", len(req.Files), tt.wantLen)
			}
		})
	}
}

func TestRunResponseJSON(t *testing.T) {
	resp := RunResponse{
		Status:     "ok",
		Stdout:     "hello\n",
		Stderr:     "",
		DurationMs: 42,
		Tests: []TestResult{
			{Name: "TestAdd", Passed: true, Output: "PASS\n"},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded RunResponse
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.Status != "ok" {
		t.Errorf("status = %q, want %q", decoded.Status, "ok")
	}
	if len(decoded.Tests) != 1 {
		t.Fatalf("tests len = %d, want 1", len(decoded.Tests))
	}
	if decoded.Tests[0].Name != "TestAdd" {
		t.Errorf("test name = %q, want %q", decoded.Tests[0].Name, "TestAdd")
	}
}
