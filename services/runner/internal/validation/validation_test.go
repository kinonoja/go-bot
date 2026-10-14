package validation

import (
	"strings"
	"testing"

	"github.com/kinonoja/gobot/services/runner/internal/types"
)

func TestValidate(t *testing.T) {
	validFiles := map[string]string{"main.go": "package main"}

	tests := []struct {
		name    string
		req     types.RunRequest
		wantErr string
	}{
		{
			name:    "valid run request",
			req:     types.RunRequest{Files: validFiles, Mode: "run", TimeoutMs: 5000},
			wantErr: "",
		},
		{
			name:    "valid test request",
			req:     types.RunRequest{Files: validFiles, Mode: "test", TimeoutMs: 5000},
			wantErr: "",
		},
		{
			name:    "invalid mode",
			req:     types.RunRequest{Files: validFiles, Mode: "exec", TimeoutMs: 5000},
			wantErr: "invalid mode",
		},
		{
			name:    "empty mode",
			req:     types.RunRequest{Files: validFiles, Mode: "", TimeoutMs: 5000},
			wantErr: "invalid mode",
		},
		{
			name:    "no files",
			req:     types.RunRequest{Files: map[string]string{}, Mode: "run", TimeoutMs: 5000},
			wantErr: "no files provided",
		},
		{
			name: "too many files",
			req: types.RunRequest{
				Files: func() map[string]string {
					m := make(map[string]string)
					for i := 0; i < 11; i++ {
						m[strings.Repeat("a", i+1)+".go"] = "package main"
					}
					return m
				}(),
				Mode:      "run",
				TimeoutMs: 5000,
			},
			wantErr: "too many files",
		},
		{
			name:    "zero timeout",
			req:     types.RunRequest{Files: validFiles, Mode: "run", TimeoutMs: 0},
			wantErr: "timeout_ms must be positive",
		},
		{
			name:    "negative timeout",
			req:     types.RunRequest{Files: validFiles, Mode: "run", TimeoutMs: -1},
			wantErr: "timeout_ms must be positive",
		},
		{
			name:    "file with path traversal",
			req:     types.RunRequest{Files: map[string]string{"../evil.go": "package main"}, Mode: "run", TimeoutMs: 5000},
			wantErr: "contains \"..\"",
		},
		{
			name:    "file with forward slash",
			req:     types.RunRequest{Files: map[string]string{"sub/main.go": "package main"}, Mode: "run", TimeoutMs: 5000},
			wantErr: "contains path separator",
		},
		{
			name:    "file with backslash",
			req:     types.RunRequest{Files: map[string]string{"sub\\main.go": "package main"}, Mode: "run", TimeoutMs: 5000},
			wantErr: "contains path separator",
		},
		{
			name:    "empty file name",
			req:     types.RunRequest{Files: map[string]string{"": "package main"}, Mode: "run", TimeoutMs: 5000},
			wantErr: "empty file name",
		},
		{
			name: "payload too large",
			req: types.RunRequest{
				Files:     map[string]string{"main.go": strings.Repeat("x", MaxPayloadSize+1)},
				Mode:      "run",
				TimeoutMs: 5000,
			},
			wantErr: "exceeds limit",
		},
		{
			name: "payload at exact limit",
			req: types.RunRequest{
				Files:     map[string]string{"main.go": strings.Repeat("x", MaxPayloadSize)},
				Mode:      "run",
				TimeoutMs: 5000,
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(&tt.req)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}
