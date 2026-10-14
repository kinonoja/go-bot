//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/kinonoja/gobot/services/runner/internal/handler"
	"github.com/kinonoja/gobot/services/runner/internal/types"
)

func TestMain(m *testing.M) {
	out, err := exec.Command("docker", "info").CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: docker info failed: %v\n%s\n", err, out)
		os.Exit(1)
	}

	out, err = exec.Command("docker", "image", "inspect", "gobot-sandbox:latest").CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: gobot-sandbox:latest image not found. Build it first:\n  docker build -t gobot-sandbox:latest services/runner/sandbox/\n%s\n", out)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func setupServer(t *testing.T) *httptest.Server {
	t.Helper()
	h := handler.New(2)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return httptest.NewServer(mux)
}

func runRequest(t *testing.T, server *httptest.Server, req types.RunRequest) types.RunResponse {
	t.Helper()
	body, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	resp, err := http.Post(server.URL+"/run", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("post error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result types.RunResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	return result
}

func noLeakedContainers(t *testing.T) {
	t.Helper()
	out, err := exec.Command("docker", "ps", "-aq", "--filter", "ancestor=gobot-sandbox:latest").Output()
	if err != nil {
		t.Fatalf("docker ps failed: %v", err)
	}
	ids := strings.TrimSpace(string(out))
	if ids != "" {
		t.Fatalf("leaked containers from gobot-sandbox:latest: %s", ids)
	}
}

func TestIntegration_ValidProgram(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	result := runRequest(t, server, types.RunRequest{
		Files: map[string]string{
			"main.go": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello gobot\")\n}\n",
		},
		Mode:      "run",
		TimeoutMs: 30000,
	})

	if result.Status != "ok" {
		t.Fatalf("status = %q, want %q\nstderr: %s", result.Status, "ok", result.Stderr)
	}
	if !strings.Contains(result.Stdout, "hello gobot") {
		t.Fatalf("stdout = %q, want it to contain %q", result.Stdout, "hello gobot")
	}
	noLeakedContainers(t)
}

func TestIntegration_CompileError(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	result := runRequest(t, server, types.RunRequest{
		Files: map[string]string{
			"main.go": "package main\n\nfunc main() {\n\tx :=\n}\n",
		},
		Mode:      "run",
		TimeoutMs: 30000,
	})

	if result.Status != "compile_error" {
		t.Fatalf("status = %q, want %q\nstderr: %s", result.Status, "compile_error", result.Stderr)
	}
	noLeakedContainers(t)
}

func TestIntegration_InfiniteLoopTimeout(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	result := runRequest(t, server, types.RunRequest{
		Files: map[string]string{
			"main.go": "package main\n\nfunc main() {\n\tfor {}\n}\n",
		},
		Mode:      "run",
		TimeoutMs: 3000,
	})

	if result.Status != "timeout" {
		t.Fatalf("status = %q, want %q", result.Status, "timeout")
	}
	noLeakedContainers(t)
}

func TestIntegration_NetworkBlocked(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	src := `package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	conn, err := net.DialTimeout("tcp", "1.1.1.1:80", 3*time.Second)
	if err != nil {
		fmt.Println("dial error:", err)
		return
	}
	conn.Close()
	fmt.Println("CONNECTION_SUCCEEDED")
}
`
	result := runRequest(t, server, types.RunRequest{
		Files:     map[string]string{"main.go": src},
		Mode:      "run",
		TimeoutMs: 30000,
	})

	if result.Status != "ok" {
		t.Fatalf("status = %q, want %q\nstderr: %s", result.Status, "ok", result.Stderr)
	}
	if strings.Contains(result.Stdout, "CONNECTION_SUCCEEDED") {
		t.Fatal("network connection succeeded, sandbox network isolation is broken")
	}
	hasDialErr := strings.Contains(result.Stdout, "dial error:")
	if !hasDialErr {
		t.Fatalf("stdout should contain a dial error, got: %q", result.Stdout)
	}
	noLeakedContainers(t)
}

func TestIntegration_ForkBomb(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	timeoutMs := 10000
	src := `package main

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
)

func main() {
	fmt.Println("started")
	os.Stdout.Sync()

	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command("sleep", "60")
			cmd.Run()
		}()
	}
	wg.Wait()
}
`
	start := time.Now()
	result := runRequest(t, server, types.RunRequest{
		Files:     map[string]string{"main.go": src},
		Mode:      "run",
		TimeoutMs: timeoutMs,
	})
	elapsed := time.Since(start)

	maxDuration := time.Duration(timeoutMs)*time.Millisecond + 3*time.Second
	if elapsed > maxDuration {
		t.Fatalf("fork bomb took %v, exceeds timeout+3s (%v)", elapsed, maxDuration)
	}

	if result.Status != "runtime_error" && result.Status != "timeout" {
		t.Fatalf("status = %q, want %q or %q", result.Status, "runtime_error", "timeout")
	}
	if !strings.Contains(result.Stdout, "started") {
		t.Fatalf("stdout = %q, want it to contain %q", result.Stdout, "started")
	}
	noLeakedContainers(t)
}

func TestIntegration_MemoryHog(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	src := `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("started")
	os.Stdout.Sync()

	var data [][]byte
	for {
		data = append(data, make([]byte, 10*1024*1024))
	}
	_ = data
}
`
	result := runRequest(t, server, types.RunRequest{
		Files:     map[string]string{"main.go": src},
		Mode:      "run",
		TimeoutMs: 30000,
	})

	if result.Status != "runtime_error" {
		t.Fatalf("status = %q, want %q\nstderr: %s", result.Status, "runtime_error", result.Stderr)
	}
	if !strings.Contains(result.Stdout, "started") {
		t.Fatalf("stdout = %q, want it to contain %q", result.Stdout, "started")
	}

	resp, err := http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatalf("healthz after memory hog failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	noLeakedContainers(t)
}

func TestIntegration_ReadOnlyFilesystem(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	src := `package main

import (
	"fmt"
	"os"
)

func main() {
	err := os.WriteFile("/workspace/hacked.txt", []byte("pwned"), 0644)
	if err != nil {
		fmt.Println("write blocked:", err)
		return
	}
	fmt.Println("WRITE_SUCCEEDED")
}
`
	result := runRequest(t, server, types.RunRequest{
		Files:     map[string]string{"main.go": src},
		Mode:      "run",
		TimeoutMs: 30000,
	})

	if result.Status != "ok" {
		t.Fatalf("status = %q, want %q\nstderr: %s", result.Status, "ok", result.Stderr)
	}
	if strings.Contains(result.Stdout, "WRITE_SUCCEEDED") {
		t.Fatal("write to read-only filesystem succeeded, sandbox isolation is broken")
	}
	if !strings.Contains(result.Stdout, "write blocked:") {
		t.Fatalf("stdout should report write error, got: %q", result.Stdout)
	}
	noLeakedContainers(t)
}

func TestIntegration_NonRootUser(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	src := `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("uid:", os.Getuid())
}
`
	result := runRequest(t, server, types.RunRequest{
		Files:     map[string]string{"main.go": src},
		Mode:      "run",
		TimeoutMs: 30000,
	})

	if result.Status != "ok" {
		t.Fatalf("status = %q, want %q\nstderr: %s", result.Status, "ok", result.Stderr)
	}
	if strings.Contains(result.Stdout, "uid: 0") {
		t.Fatal("process running as root (uid 0), sandbox should use non-root user")
	}
	if !strings.Contains(result.Stdout, "uid:") {
		t.Fatalf("stdout should contain uid, got: %q", result.Stdout)
	}
	noLeakedContainers(t)
}

func TestIntegration_TestMode(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	result := runRequest(t, server, types.RunRequest{
		Files: map[string]string{
			"main.go":      "package main\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n",
			"main_test.go": "package main\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 3) != 5 {\n\t\tt.Error(\"expected 5\")\n\t}\n}\n\nfunc TestAddNegative(t *testing.T) {\n\tif Add(-1, 1) != 0 {\n\t\tt.Error(\"expected 0\")\n\t}\n}\n",
		},
		Mode:      "test",
		TimeoutMs: 30000,
	})

	if result.Status != "ok" {
		t.Fatalf("status = %q, want %q\nstderr: %s", result.Status, "ok", result.Stderr)
	}
	if len(result.Tests) == 0 {
		t.Fatal("expected test results")
	}
	for _, tr := range result.Tests {
		if !tr.Passed {
			t.Errorf("test %q failed", tr.Name)
		}
	}
	noLeakedContainers(t)
}
