package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateWorkspacePermissions(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
	}{
		{
			name:  "single submitted file",
			files: map[string]string{"main.go": "package main"},
		},
		{
			name: "multiple submitted files",
			files: map[string]string{
				"main.go":      "package main",
				"main_test.go": "package main",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workDir, err := createWorkspace(tt.files)
			if err != nil {
				t.Fatalf("createWorkspace() error = %v", err)
			}
			defer os.RemoveAll(workDir)

			info, err := os.Stat(workDir)
			if err != nil {
				t.Fatalf("stat workspace: %v", err)
			}
			if got := info.Mode().Perm(); got != 0755 {
				t.Errorf("workspace permissions = %04o, want 0755", got)
			}

			for name := range tt.files {
				assertWorkspaceFilePermissions(t, workDir, name)
			}
			assertWorkspaceFilePermissions(t, workDir, "go.mod")
		})
	}
}

func assertWorkspaceFilePermissions(t *testing.T, workDir, name string) {
	t.Helper()

	info, err := os.Stat(filepath.Join(workDir, name))
	if err != nil {
		t.Fatalf("stat workspace file %s: %v", name, err)
	}
	if got := info.Mode().Perm(); got != 0644 {
		t.Errorf("permissions for %s = %04o, want 0644", name, got)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLen   int
		wantTrunc bool
	}{
		{
			name:      "short string unchanged",
			input:     "hello",
			wantLen:   5,
			wantTrunc: false,
		},
		{
			name:      "exact limit unchanged",
			input:     strings.Repeat("x", MaxOutputBytes),
			wantLen:   MaxOutputBytes,
			wantTrunc: false,
		},
		{
			name:      "over limit gets truncated",
			input:     strings.Repeat("x", MaxOutputBytes+100),
			wantLen:   MaxOutputBytes + len(TruncationMsg),
			wantTrunc: true,
		},
		{
			name:      "empty string",
			input:     "",
			wantLen:   0,
			wantTrunc: false,
		},
		{
			name:      "one byte over limit",
			input:     strings.Repeat("x", MaxOutputBytes+1),
			wantLen:   MaxOutputBytes + len(TruncationMsg),
			wantTrunc: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Truncate(tt.input)
			if len(result) != tt.wantLen {
				t.Errorf("len = %d, want %d", len(result), tt.wantLen)
			}
			hasTrunc := strings.HasSuffix(result, TruncationMsg)
			if hasTrunc != tt.wantTrunc {
				t.Errorf("truncated = %v, want %v", hasTrunc, tt.wantTrunc)
			}
		})
	}
}
