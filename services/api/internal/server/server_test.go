package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kinonoja/gobot/services/api/internal/lessons"
	"github.com/kinonoja/gobot/services/api/internal/runnerclient"
	"github.com/kinonoja/gobot/services/api/internal/store"
)

type fakeRunnerState struct {
	response runnerclient.Response
	status   int
	request  runnerclient.Request
}

func newServerFixture(t *testing.T, devAuth bool, state *fakeRunnerState) (http.Handler, *store.Memory, *httptest.Server) {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go-course", "course.yaml"), "title: Go Course\ndescription: Learn Go\n")
	chapter := filepath.Join(root, "go-course", "01-start")
	writeTestFile(t, filepath.Join(chapter, "chapter.yaml"), "title: Start\ndescription: Begin\n")
	lesson := filepath.Join(chapter, "01-hello")
	writeTestFile(t, filepath.Join(lesson, "lesson.md"), "---\ntitle: Hello\nxp: 20\ndifficulty: easy\nexpected_tests:\n  - TestHello\n  - TestGreeting\n---\n\n# Goal\n")
	writeTestFile(t, filepath.Join(lesson, "starter", "main.go"), "package main\nfunc Greeting() string { return \"TODO\" }\n")
	writeTestFile(t, filepath.Join(lesson, "solution", "main.go"), "package main\nfunc Greeting() string { return \"solution-secret\" }\n")
	writeTestFile(t, filepath.Join(lesson, "tests", "main_test.go"), "package main\nimport \"testing\"\nconst graderSecret = \"grader-secret\"\nfunc TestHello(t *testing.T) {}\nfunc TestGreeting(t *testing.T) {}\n")
	courses, err := lessons.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	runnerServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if state.status != 0 {
			w.WriteHeader(state.status)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&state.request); err != nil {
			t.Errorf("decode runner request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(state.response)
	}))
	dataStore := store.NewMemory()
	api := NewHandler(courses, dataStore, runnerclient.New(runnerServer.URL, time.Second), devAuth)
	return api, dataStore, runnerServer
}

func writeTestFile(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestPublicEndpoints(t *testing.T) {
	state := &fakeRunnerState{}
	api, _, server := newServerFixture(t, true, state)
	defer server.Close()
	tests := []struct {
		name   string
		path   string
		status int
		check  func(*testing.T, string)
	}{
		{name: "health", path: "/healthz", status: http.StatusOK, check: func(t *testing.T, body string) {
			if !strings.Contains(body, `"status":"ok"`) {
				t.Fatalf("body = %s", body)
			}
		}},
		{name: "course catalog", path: "/api/courses", status: http.StatusOK, check: func(t *testing.T, body string) {
			if !strings.Contains(body, "Go Course") || strings.Contains(body, "solution-secret") || strings.Contains(body, "grader-secret") {
				t.Fatalf("unexpected course response: %s", body)
			}
		}},
		{name: "lesson has no hidden files", path: "/api/courses/go-course/lessons/start/hello", status: http.StatusOK, check: func(t *testing.T, body string) {
			if !strings.Contains(body, "TODO") || strings.Contains(body, "solution-secret") || strings.Contains(body, "grader-secret") {
				t.Fatalf("unexpected lesson response: %s", body)
			}
		}},
		{name: "unknown lesson", path: "/api/courses/go-course/lessons/start/missing", status: http.StatusNotFound, check: func(t *testing.T, body string) {
			if !strings.Contains(body, `"error"`) {
				t.Fatalf("body = %s", body)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			api.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.status, response.Body.String())
			}
			if response.Header().Get("X-Request-ID") == "" || response.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("missing response middleware headers: %v", response.Header())
			}
			test.check(t, response.Body.String())
		})
	}
}

func TestSubmit(t *testing.T) {
	tests := []struct {
		name           string
		response       runnerclient.Response
		runnerStatus   int
		body           string
		devAuth        bool
		withUser       bool
		wantHTTP       int
		wantPassed     bool
		wantXP         int
		wantNew        bool
		wantRetryAfter string
		wantError      string
	}{
		{name: "passing", response: okResponse(), body: `{"files":{"main.go":"package main"}}`, devAuth: true, withUser: true, wantHTTP: http.StatusOK, wantPassed: true, wantXP: 22, wantNew: true},
		{name: "duplicate result with failure cannot pass", response: runnerclient.Response{Status: "ok", Tests: []runnerclient.TestResult{{Name: "TestHello", Passed: true}, {Name: "TestHello", Passed: false}, {Name: "TestGreeting", Passed: true}}, MissingTests: []string{}}, body: `{"files":{"main.go":"package main"}}`, devAuth: true, withUser: true, wantHTTP: http.StatusOK},
		{name: "failing", response: runnerclient.Response{Status: "runtime_error", Tests: []runnerclient.TestResult{{Name: "TestHello", Passed: false, Output: strings.Repeat("x", 2100)}}, MissingTests: []string{"TestGreeting"}}, body: `{"files":{"main.go":"package main"}}`, devAuth: true, withUser: true, wantHTTP: http.StatusOK, wantError: "runtime_error"},
		{name: "compile error", response: runnerclient.Response{Status: "compile_error", Stderr: strings.Repeat("e", 4100), MissingTests: []string{"TestHello", "TestGreeting"}}, body: `{"files":{"main.go":"bad"}}`, devAuth: true, withUser: true, wantHTTP: http.StatusOK, wantError: "compile_error"},
		{name: "missing tests prevents pass", response: runnerclient.Response{Status: "ok", Tests: []runnerclient.TestResult{{Name: "TestHello", Passed: true}, {Name: "TestGreeting", Passed: true}}, MissingTests: []string{"TestGreeting"}}, body: `{"files":{"main.go":"package main"}}`, devAuth: true, withUser: true, wantHTTP: http.StatusOK, wantError: "missing_tests"},
		{name: "runner rate limit", runnerStatus: http.StatusTooManyRequests, body: `{"files":{"main.go":"package main"}}`, devAuth: true, withUser: true, wantHTTP: http.StatusServiceUnavailable, wantRetryAfter: "2", wantError: "runner is busy"},
		{name: "bad JSON", response: okResponse(), body: `{"files":`, devAuth: true, withUser: true, wantHTTP: http.StatusBadRequest, wantError: "invalid request JSON"},
		{name: "oversized body", response: okResponse(), body: fmt.Sprintf(`{"files":{"main.go":"%s"}}`, strings.Repeat("x", maxSubmitBody)), devAuth: true, withUser: true, wantHTTP: http.StatusBadRequest, wantError: "invalid request JSON"},
		{name: "missing user", response: okResponse(), body: `{"files":{"main.go":"package main"}}`, devAuth: true, wantHTTP: http.StatusUnauthorized, wantError: "authentication required"},
		{name: "development auth disabled", response: okResponse(), body: `{"files":{"main.go":"package main"}}`, devAuth: false, withUser: true, wantHTTP: http.StatusUnauthorized, wantError: "authentication required"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := &fakeRunnerState{response: test.response, status: test.runnerStatus}
			api, _, runner := newServerFixture(t, test.devAuth, state)
			defer runner.Close()
			request := httptest.NewRequest(http.MethodPost, "/api/courses/go-course/lessons/start/hello/submit", strings.NewReader(test.body))
			request.Header.Set("Content-Type", "application/json")
			if test.withUser {
				request.Header.Set("X-Dev-User-Email", "learner@example.test")
			}
			response := httptest.NewRecorder()
			api.ServeHTTP(response, request)
			if response.Code != test.wantHTTP {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.wantHTTP, response.Body.String())
			}
			if test.wantError != "" && !strings.Contains(response.Body.String(), test.wantError) {
				t.Fatalf("body = %s, want it to contain %q", response.Body.String(), test.wantError)
			}
			if test.wantRetryAfter != "" && response.Header().Get("Retry-After") != test.wantRetryAfter {
				t.Fatalf("Retry-After = %q, want %q", response.Header().Get("Retry-After"), test.wantRetryAfter)
			}
			if test.wantHTTP == http.StatusOK {
				var result submitResponse
				if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result.Passed != test.wantPassed || result.XPAwarded != test.wantXP || result.NewlyCompleted != test.wantNew {
					t.Fatalf("response = %+v", result)
				}
				if test.name == "failing" && len(result.Tests) != 1 || test.name == "failing" && len([]rune(result.Tests[0].Output)) != 2000 {
					t.Fatalf("failed test output was not truncated: %+v", result.Tests)
				}
				if test.name == "compile error" && len([]rune(result.Stderr)) != 4000 {
					t.Fatalf("compile stderr length = %d", len([]rune(result.Stderr)))
				}
				if test.name == "passing" {
					if len(state.request.GraderFiles) != 1 || len(state.request.ExpectedTests) != 2 || state.request.Mode != "test" {
						t.Fatalf("runner request = %+v", state.request)
					}
					if strings.Contains(response.Body.String(), "grader-secret") || result.Tests[0].Output != "" {
						t.Fatalf("successful response leaked output or grader data: %s", response.Body.String())
					}
				}
			}
		})
	}
}

func TestRunnerDownAndRateLimit(t *testing.T) {
	state := &fakeRunnerState{response: okResponse()}
	api, _, runner := newServerFixture(t, true, state)
	request := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/courses/go-course/lessons/start/hello/submit", strings.NewReader(`{"files":{"main.go":"package main"}}`))
		req.Header.Set("X-Dev-User-Email", "learner@example.test")
		response := httptest.NewRecorder()
		api.ServeHTTP(response, req)
		return response
	}
	first := request()
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d: %s", first.Code, first.Body.String())
	}
	second := request()
	if second.Code != http.StatusTooManyRequests || !strings.Contains(second.Body.String(), "rate limit") {
		t.Fatalf("second response = %d %s", second.Code, second.Body.String())
	}
	runner.Close()

	closedURL := "http://127.0.0.1:1"
	courses := fixtureCourses(t)
	downHandler := NewHandler(courses, store.NewMemory(), runnerclient.New(closedURL, 50*time.Millisecond), true)
	downReq := httptest.NewRequest(http.MethodPost, "/api/courses/go-course/lessons/start/hello/submit", strings.NewReader(`{"files":{"main.go":"package main"}}`))
	downReq.Header.Set("X-Dev-User-Email", "learner@example.test")
	downResponse := httptest.NewRecorder()
	downHandler.ServeHTTP(downResponse, downReq)
	if downResponse.Code != http.StatusBadGateway || !strings.Contains(downResponse.Body.String(), "runner is unavailable") {
		t.Fatalf("runner down response = %d %s", downResponse.Code, downResponse.Body.String())
	}
}

func TestProgressTotalsAndRepeatCompletion(t *testing.T) {
	state := &fakeRunnerState{response: okResponse()}
	api, _, runner := newServerFixture(t, true, state)
	defer runner.Close()
	submit := func() submitResponse {
		request := httptest.NewRequest(http.MethodPost, "/api/courses/go-course/lessons/start/hello/submit", strings.NewReader(`{"files":{"main.go":"package main"}}`))
		request.Header.Set("X-Dev-User-Email", "learner@example.test")
		response := httptest.NewRecorder()
		api.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("submit status = %d: %s", response.Code, response.Body.String())
		}
		var result submitResponse
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	first := submit()
	if !first.NewlyCompleted || first.XPAwarded != 22 {
		t.Fatalf("first completion = %+v", first)
	}
	time.Sleep(userSubmitInterval + 5*time.Millisecond)
	second := submit()
	if second.NewlyCompleted || second.XPAwarded != 0 {
		t.Fatalf("repeat completion = %+v", second)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/me/progress", nil)
	request.Header.Set("X-Dev-User-Email", "learner@example.test")
	response := httptest.NewRecorder()
	api.ServeHTTP(response, request)
	var progress progressResponse
	if err := json.Unmarshal(response.Body.Bytes(), &progress); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || progress.TotalXP != 22 || len(progress.Completed) != 1 {
		t.Fatalf("progress = status %d, %+v", response.Code, progress)
	}
}

func TestProgressRequiresUser(t *testing.T) {
	api, _, runner := newServerFixture(t, false, &fakeRunnerState{})
	defer runner.Close()
	request := httptest.NewRequest(http.MethodGet, "/api/me/progress", nil)
	response := httptest.NewRecorder()
	api.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func fixtureCourses(t *testing.T) []lessons.Course {
	t.Helper()
	return loadFixtureCourses(t)
}

func loadFixtureCourses(t *testing.T) []lessons.Course {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go-course", "course.yaml"), "title: Go Course\ndescription: Learn Go\n")
	chapter := filepath.Join(root, "go-course", "01-start")
	writeTestFile(t, filepath.Join(chapter, "chapter.yaml"), "title: Start\ndescription: Begin\n")
	lesson := filepath.Join(chapter, "01-hello")
	writeTestFile(t, filepath.Join(lesson, "lesson.md"), "---\ntitle: Hello\nxp: 20\ndifficulty: easy\nexpected_tests:\n  - TestHello\n  - TestGreeting\n---\n\n# Goal\n")
	writeTestFile(t, filepath.Join(lesson, "starter", "main.go"), "package main\nfunc Greeting() string { return \"TODO\" }\n")
	writeTestFile(t, filepath.Join(lesson, "solution", "main.go"), "package main\nfunc Greeting() string { return \"solution-secret\" }\n")
	writeTestFile(t, filepath.Join(lesson, "tests", "main_test.go"), "package main\nimport \"testing\"\nfunc TestHello(t *testing.T) {}\nfunc TestGreeting(t *testing.T) {}\n")
	courses, err := lessons.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return courses
}

func okResponse() runnerclient.Response {
	return runnerclient.Response{Status: "ok", Tests: []runnerclient.TestResult{{Name: "TestHello", Passed: true, Output: "success"}, {Name: "TestGreeting", Passed: true, Output: "success"}}, MissingTests: []string{}, DurationMs: 30}
}

func TestErrorResponsesAreValidJSON(t *testing.T) {
	api, _, runner := newServerFixture(t, false, &fakeRunnerState{})
	defer runner.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/courses/go-course/lessons/start/hello/submit", bytes.NewReader([]byte("{}")))
	response := httptest.NewRecorder()
	api.ServeHTTP(response, request)
	var value map[string]string
	data, _ := io.ReadAll(response.Body)
	if err := json.Unmarshal(data, &value); err != nil || value["error"] == "" {
		t.Fatalf("error response is not consistent JSON: %s", data)
	}
}
