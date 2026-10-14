package sandbox

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
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

func TestContainerArgs(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want []string
	}{
		{
			name: "run mode",
			mode: "run",
			want: []string{
				"run",
				"--name", "gobot-run-123",
				"--network", "none",
				"--memory", "512m",
				"--cpus", "0.5",
				"--pids-limit", "64",
				"--read-only",
				"--cap-drop", "ALL",
				"--security-opt", "no-new-privileges",
				"--user", "sandbox",
				"--tmpfs", "/tmp:rw,exec,size=384m",
				"-e", "GOCACHE=/tmp/gocache",
				"-e", "GOPROXY=off",
				"-e", "HOME=/tmp",
				"-v", "/tmp/gobot-run-test:/workspace:ro",
				"-w", "/workspace",
				SandboxImage,
				"sh", "-c", "cp -r /opt/gocache /tmp/gocache && { go build -o /tmp/submission . || exit 97; } && exec /tmp/submission",
			},
		},
		{
			name: "test mode",
			mode: "test",
			want: []string{
				"run",
				"--name", "gobot-run-123",
				"--network", "none",
				"--memory", "512m",
				"--cpus", "0.5",
				"--pids-limit", "64",
				"--read-only",
				"--cap-drop", "ALL",
				"--security-opt", "no-new-privileges",
				"--user", "sandbox",
				"--tmpfs", "/tmp:rw,exec,size=384m",
				"-e", "GOCACHE=/tmp/gocache",
				"-e", "GOPROXY=off",
				"-e", "HOME=/tmp",
				"-v", "/tmp/gobot-run-test:/workspace:ro",
				"-w", "/workspace",
				SandboxImage,
				"sh", "-c", "cp -r /opt/gocache /tmp/gocache && { go test -c -o /tmp/submission.test . || exit 97; } && exec go tool test2json -p submission -t /tmp/submission.test -test.v",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containerArgs("gobot-run-123", "/tmp/gobot-run-test", tt.mode)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("containerArgs() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestStatusForExitCode(t *testing.T) {
	tests := []struct {
		name string
		code int
		mode string
		want string
	}{
		{name: "run build failure", code: 97, mode: "run", want: "compile_error"},
		{name: "test build failure", code: 97, mode: "test", want: "compile_error"},
		{name: "run code 1 remains runtime error", code: 1, mode: "run", want: "runtime_error"},
		{name: "test failure remains ok status", code: 1, mode: "test", want: "ok"},
		{name: "run code 2 remains compile error", code: 2, mode: "run", want: "compile_error"},
		{name: "test code 2 remains compile error", code: 2, mode: "test", want: "compile_error"},
		{name: "run unknown code remains runtime error", code: 3, mode: "run", want: "runtime_error"},
		{name: "test unknown code remains runtime error", code: 3, mode: "test", want: "runtime_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statusForExitCode(tt.code, tt.mode); got != tt.want {
				t.Errorf("statusForExitCode(%d, %q) = %q, want %q", tt.code, tt.mode, got, tt.want)
			}
		})
	}
}

func TestExecutionTimeout(t *testing.T) {
	tests := []struct {
		name      string
		timeoutMs int
		want      time.Duration
	}{
		{name: "zero request timeout", timeoutMs: 0, want: 10 * time.Second},
		{name: "one millisecond request timeout", timeoutMs: 1, want: 10*time.Second + time.Millisecond},
		{name: "five second request timeout", timeoutMs: 5000, want: 15 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := executionTimeout(tt.timeoutMs); got != tt.want {
				t.Errorf("executionTimeout(%d) = %v, want %v", tt.timeoutMs, got, tt.want)
			}
		})
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
