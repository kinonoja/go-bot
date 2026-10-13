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
	workDir, err := os.MkdirTemp("", "gobot-run-*")
	if err != nil {
		return &types.RunResponse{
			Status: "runtime_error",
			Stderr: fmt.Sprintf("failed to create temp dir: %v", err),
		}
	}
	defer os.RemoveAll(workDir)

	goMod := "module submission\n\ngo 1.23\n"
	if err := os.WriteFile(filepath.Join(workDir, "go.mod"), []byte(goMod), 0644); err != nil {
		return &types.RunResponse{
			Status: "runtime_error",
			Stderr: fmt.Sprintf("failed to write go.mod: %v", err),
		}
	}

	for name, content := range req.Files {
		if err := os.WriteFile(filepath.Join(workDir, name), []byte(content), 0644); err != nil {
			return &types.RunResponse{
				Status: "runtime_error",
				Stderr: fmt.Sprintf("failed to write file %s: %v", name, err),
			}
		}
	}

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
		"--rm",
		"--network", "none",
		"--memory", "256m",
		"--cpus", "0.5",
		"--pids-limit", "64",
		"--read-only",
		"--tmpfs", "/tmp:rw,noexec,nosuid,size=64m",
		"--tmpfs", "/home/sandbox/.cache:rw,noexec,nosuid,size=128m",
		"--cap-drop", "ALL",
		"--security-opt", "no-new-privileges",
		"-v", workDir + ":/workspace:ro",
		"-w", "/workspace",
		SandboxImage,
		"sh", "-c", goCmd,
	}

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
		if ok && exitErr.ExitCode() == 1 && req.Mode == "test" {
			resp.Status = "ok"
		} else if ok && exitErr.ExitCode() == 2 {
			resp.Status = "compile_error"
		} else if ok && exitErr.ExitCode() == 1 {
			resp.Status = "compile_error"
		} else {
			resp.Status = "runtime_error"
		}
	} else {
		resp.Status = "ok"
	}

	return resp
}

func killContainer(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = exec.CommandContext(ctx, "docker", "kill", name).Run()
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
