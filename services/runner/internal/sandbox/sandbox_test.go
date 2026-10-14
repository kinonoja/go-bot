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
		name         string
		studentFiles map[string]string
		graderFiles  map[string]string
	}{
		{
			name:         "single student file",
			studentFiles: map[string]string{"main.go": "package main"},
		},
		{
			name: "student and grader files",
			studentFiles: map[string]string{
				"main.go": "package main",
			},
			graderFiles: map[string]string{
				"grader_test.go": "package main",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workDir, err := createWorkspace(tt.studentFiles, tt.graderFiles)
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

			for name := range tt.studentFiles {
				assertWorkspaceFilePermissions(t, workDir, name)
			}
			for name := range tt.graderFiles {
				assertWorkspaceFilePermissions(t, workDir, name)
			}
			assertWorkspaceFilePermissions(t, workDir, "go.mod")
		})
	}
}

func TestWorkspaceFilesDoNotOverwriteExistingFiles(t *testing.T) {
	workDir := t.TempDir()
	studentFiles := map[string]string{"grader_test.go": "student"}
	graderFiles := map[string]string{"grader_test.go": "grader"}

	if err := writeWorkspaceFiles(workDir, studentFiles); err != nil {
		t.Fatalf("write student file: %v", err)
	}
	if err := writeWorkspaceFiles(workDir, graderFiles); err == nil {
		t.Fatal("write grader file over existing file: got nil error")
	}

	content, err := os.ReadFile(filepath.Join(workDir, "grader_test.go"))
	if err != nil {
		t.Fatalf("read existing file: %v", err)
	}
	if string(content) != "student" {
		t.Fatalf("file content = %q, want %q", content, "student")
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

func TestStatusForExecution(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
		stderr   string
		timedOut bool
		want     string
	}{
		{name: "exit 0 without stderr", exitCode: 0, want: "ok"},
		{name: "exit 0 with syntax stderr", exitCode: 0, stderr: "syntax error", want: "ok"},
		{name: "exit 1 without stderr", exitCode: 1, want: "runtime_error"},
		{name: "exit 1 with fatal stderr", exitCode: 1, stderr: "fatal error", want: "runtime_error"},
		{name: "exit 2 without stderr", exitCode: 2, want: "runtime_error"},
		{name: "exit 2 with fatal stderr", exitCode: 2, stderr: "fatal error", want: "runtime_error"},
		{name: "exit 97 without stderr", exitCode: 97, want: "compile_error"},
		{name: "exit 97 with syntax stderr", exitCode: 97, stderr: "syntax error", want: "compile_error"},
		{name: "exit 125 without stderr", exitCode: 125, want: "runtime_error"},
		{name: "exit 125 with fatal stderr", exitCode: 125, stderr: "fatal error", want: "runtime_error"},
		{name: "exit 126 without stderr", exitCode: 126, want: "runtime_error"},
		{name: "exit 126 with syntax stderr", exitCode: 126, stderr: "syntax error", want: "runtime_error"},
		{name: "exit 127 without stderr", exitCode: 127, want: "runtime_error"},
		{name: "exit 127 with fatal stderr", exitCode: 127, stderr: "fatal error", want: "runtime_error"},
		{name: "exit 137 without stderr", exitCode: 137, want: "runtime_error"},
		{name: "exit 137 with syntax stderr", exitCode: 137, stderr: "syntax error", want: "runtime_error"},
		{name: "exit 139 without stderr", exitCode: 139, want: "runtime_error"},
		{name: "exit 139 with fatal stderr", exitCode: 139, stderr: "fatal error", want: "runtime_error"},
		{name: "exit 143 without stderr", exitCode: 143, want: "runtime_error"},
		{name: "exit 143 with syntax stderr", exitCode: 143, stderr: "syntax error", want: "runtime_error"},
		{name: "timeout overrides exit code and stderr", exitCode: 137, stderr: "fatal error", timedOut: true, want: "timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := statusForExecution(tt.exitCode, tt.stderr, tt.timedOut); got != tt.want {
				t.Errorf("statusForExecution(%d, %q, %v) = %q, want %q", tt.exitCode, tt.stderr, tt.timedOut, got, tt.want)
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
