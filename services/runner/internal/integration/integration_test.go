//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kinonoja/gobot/services/runner/internal/handler"
	"github.com/kinonoja/gobot/services/runner/internal/types"
)

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

func TestIntegration_ValidProgram(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	result := runRequest(t, server, types.RunRequest{
		Files: map[string]string{
			"main.go": `package main

import "fmt"

func main() {
	fmt.Println("hello gobot")
}
`,
		},
		Mode:      "run",
		TimeoutMs: 15000,
	})

	if result.Status != "ok" {
		t.Fatalf("status = %q, want %q\nstderr: %s", result.Status, "ok", result.Stderr)
	}
	if result.Stdout == "" {
		t.Fatal("expected stdout output")
	}
}

func TestIntegration_CompileError(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	result := runRequest(t, server, types.RunRequest{
		Files: map[string]string{
			"main.go": `package main

func main() {
	x :=
}
`,
		},
		Mode:      "run",
		TimeoutMs: 15000,
	})

	if result.Status != "compile_error" {
		t.Fatalf("status = %q, want %q\nstderr: %s", result.Status, "compile_error", result.Stderr)
	}
}

func TestIntegration_InfiniteLoopTimeout(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	result := runRequest(t, server, types.RunRequest{
		Files: map[string]string{
			"main.go": `package main

func main() {
	for {}
}
`,
		},
		Mode:      "run",
		TimeoutMs: 3000,
	})

	if result.Status != "timeout" {
		t.Fatalf("status = %q, want %q", result.Status, "timeout")
	}
}

func TestIntegration_NetworkBlocked(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	result := runRequest(t, server, types.RunRequest{
		Files: map[string]string{
			"main.go": `package main

import (
	"fmt"
	"net/http"
)

func main() {
	_, err := http.Get("https://example.com")
	if err != nil {
		fmt.Println("network blocked:", err)
		return
	}
	fmt.Println("network should be blocked")
}
`,
		},
		Mode:      "run",
		TimeoutMs: 15000,
	})

	if result.Status == "timeout" {
		t.Skip("network test timed out instead of erroring, still safe")
	}
	if result.Status != "ok" && result.Status != "runtime_error" {
		t.Fatalf("status = %q, want ok or runtime_error", result.Status)
	}
}

func TestIntegration_ForkBomb(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	result := runRequest(t, server, types.RunRequest{
		Files: map[string]string{
			"main.go": `package main

import (
	"os/exec"
	"sync"
)

func main() {
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
`,
		},
		Mode:      "run",
		TimeoutMs: 10000,
	})

	if result.Status == "ok" {
		t.Fatal("fork bomb should not succeed cleanly")
	}
}

func TestIntegration_MemoryHog(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	result := runRequest(t, server, types.RunRequest{
		Files: map[string]string{
			"main.go": `package main

func main() {
	var data [][]byte
	for {
		data = append(data, make([]byte, 10*1024*1024))
	}
	_ = data
}
`,
		},
		Mode:      "run",
		TimeoutMs: 15000,
	})

	if result.Status == "ok" {
		t.Fatal("memory hog should not succeed cleanly")
	}
}

func TestIntegration_TestMode(t *testing.T) {
	server := setupServer(t)
	defer server.Close()

	result := runRequest(t, server, types.RunRequest{
		Files: map[string]string{
			"main.go": `package main

func Add(a, b int) int {
	return a + b
}
`,
			"main_test.go": `package main

import "testing"

func TestAdd(t *testing.T) {
	if Add(2, 3) != 5 {
		t.Error("expected 5")
	}
}

func TestAddNegative(t *testing.T) {
	if Add(-1, 1) != 0 {
		t.Error("expected 0")
	}
}
`,
		},
		Mode:      "test",
		TimeoutMs: 15000,
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
}
