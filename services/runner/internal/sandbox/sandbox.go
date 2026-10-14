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
	setupAllowance = 10 * time.Second
	runCommand     = "cp -r /opt/gocache /tmp/gocache && exec go run ."
	testCommand    = "cp -r /opt/gocache /tmp/gocache && exec go test -json ./..."
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

	runCtx, cancel := context.WithTimeout(ctx, executionTimeout(req.TimeoutMs))
	defer cancel()

	containerName := fmt.Sprintf("gobot-run-%d", time.Now().UnixNano())
	defer removeContainer(containerName)

	args := containerArgs(containerName, workDir, req.Mode)

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

func containerArgs(containerName, workDir, mode string) []string {
	command := runCommand
	if mode == "test" {
		command = testCommand
	}

	return []string{
		"run",
		"--name", containerName,
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
		"-v", workDir + ":/workspace:ro",
		"-w", "/workspace",
		SandboxImage,
		"sh", "-c", command,
	}
}

func executionTimeout(timeoutMs int) time.Duration {
	return time.Duration(timeoutMs)*time.Millisecond + setupAllowance
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
