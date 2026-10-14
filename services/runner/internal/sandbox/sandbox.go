package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/kinonoja/gobot/services/runner/internal/types"
)

const (
	MaxOutputBytes = 64 * 1024
	TruncationMsg  = "\n... output truncated ..."
	SandboxImage   = "gobot-sandbox:latest"
)

func Execute(ctx context.Context, req *types.RunRequest) *types.RunResponse {
	workDir, err := createWorkspace(req.Files)
	if err != nil {
		return &types.RunResponse{
			Status: "runtime_error",
			Stderr: fmt.Sprintf("failed to create temp dir: %v", err),
		}
	}
	defer os.RemoveAll(workDir)

	timeout := time.Duration(req.TimeoutMs) * time.Millisecond
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var goCmd string
	switch req.Mode {
	case "test":
		goCmd = "go test -json ./..."
	default:
		goCmd = "go run ."
	}

	containerName := fmt.Sprintf("gobot-run-%d", time.Now().UnixNano())

	args := []string{
		"run",
		"--name", containerName,
		"--network", "none",
		"--memory", "256m",
		"--cpus", "0.5",
		"--pids-limit", "64",
		"--read-only",
		"--tmpfs", "/tmp:rw,exec,nosuid,mode=1777,size=64m",
		"--tmpfs", "/home/sandbox/.cache:rw,exec,nosuid,mode=1777,size=128m",
		"--env", "GOCACHE=/home/sandbox/.cache/go-build",
		"--env", "GOTMPDIR=/tmp",
		"--env", "HOME=/tmp",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"-v", workDir + ":/workspace:ro",
		"-w", "/workspace",
		SandboxImage,
		"sh", "-c", goCmd,
	}

	defer removeContainer(containerName)

	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(runCtx, "docker", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	if runCtx.Err() == context.DeadlineExceeded {
		killContainer(containerName)
		return &types.RunResponse{
			Status:     "timeout",
			Stdout:     truncate(stdout.String()),
			Stderr:     truncate(stderr.String()),
			DurationMs: duration.Milliseconds(),
		}
	}

	resp := &types.RunResponse{
		Stdout:     truncate(stdout.String()),
		Stderr:     truncate(stderr.String()),
		DurationMs: duration.Milliseconds(),
	}

	if runErr != nil {
		exitErr, ok := runErr.(*exec.ExitError)
		if !ok {
			resp.Status = "runtime_error"
		} else {
			code := exitErr.ExitCode()
			switch {
			case code == 1 && req.Mode == "test":
				resp.Status = "ok"
			case code == 2:
				resp.Status = "compile_error"
			default:
				resp.Status = "runtime_error"
			}
		}
	} else {
		resp.Status = "ok"
	}

	return resp
}

func createWorkspace(files map[string]string) (string, error) {
	workDir, err := os.MkdirTemp("", "gobot-run-*")
	if err != nil {
		return "", err
	}

	if err := os.Chmod(workDir, 0755); err != nil {
		os.RemoveAll(workDir)
		return "", fmt.Errorf("failed to chmod temp dir: %w", err)
	}

	workspaceFiles := make(map[string]string, len(files)+1)
	workspaceFiles["go.mod"] = "module submission\n\ngo 1.23\n"
	for name, content := range files {
		workspaceFiles[name] = content
	}

	for name, content := range workspaceFiles {
		path := filepath.Join(workDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			os.RemoveAll(workDir)
			return "", fmt.Errorf("failed to write file %s: %w", name, err)
		}
		if err := os.Chmod(path, 0644); err != nil {
			os.RemoveAll(workDir)
			return "", fmt.Errorf("failed to chmod file %s: %w", name, err)
		}
	}

	return workDir, nil
}

func killContainer(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, "docker", "kill", name).Run()
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", name).Run()
}

func removeContainer(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, "docker", "rm", "-f", name).Run()
}

func Truncate(s string) string {
	return truncate(s)
}

func truncate(s string) string {
	if len(s) <= MaxOutputBytes {
		return s
	}
	return s[:MaxOutputBytes] + TruncationMsg
}
