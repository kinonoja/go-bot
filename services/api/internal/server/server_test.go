package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
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
		{name: "runner error", response: runnerclient.Response{Status: "runner_error"}, body: `{"files":{"main.go":"package main"}}`, devAuth: true, withUser: true, wantHTTP: http.StatusOK, wantError: "runner_error"},
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
				var raw map[string]json.RawMessage
				if err := json.Unmarshal(response.Body.Bytes(), &raw); err != nil {
					t.Fatal(err)
				}
				for _, field := range []string{"missing_tests", "tests"} {
					var values []json.RawMessage
					if err := json.Unmarshal(raw[field], &values); err != nil || values == nil {
						t.Fatalf("%s = %s, want JSON array", field, raw[field])
					}
				}
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

type failingTestStore struct {
	base    *store.Memory
	failAt  string
	failure error
}

func newFailingTestStore(failAt string) *failingTestStore {
	return &failingTestStore{base: store.NewMemory(), failAt: failAt, failure: errors.New("database sentinel failure")}
}

func (s *failingTestStore) UpsertUser(ctx context.Context, email, displayName string) (store.User, error) {
	if s.failAt == "upsert_user" {
		return store.User{}, s.failure
	}
	return s.base.UpsertUser(ctx, email, displayName)
}

func (s *failingTestStore) RecordSubmission(ctx context.Context, submission store.Submission) error {
	if s.failAt == "record_submission" {
		return s.failure
	}
	return s.base.RecordSubmission(ctx, submission)
}

func (s *failingTestStore) CompleteLesson(ctx context.Context, userID string, key store.LessonKey, xp int) (bool, int, error) {
	if s.failAt == "complete_lesson" {
		return false, 0, s.failure
	}
	return s.base.CompleteLesson(ctx, userID, key, xp)
}

func (s *failingTestStore) ReadProgress(ctx context.Context, userID string) ([]store.Progress, error) {
	if s.failAt == "read_progress" {
		return nil, s.failure
	}
	return s.base.ReadProgress(ctx, userID)
}

func (s *failingTestStore) TotalXP(ctx context.Context, userID string) (int, error) {
	if s.failAt == "total_xp" {
		return 0, s.failure
	}
	return s.base.TotalXP(ctx, userID)
}

type runnerStub struct {
	response runnerclient.Response
	err      error
}

func (s runnerStub) Run(context.Context, runnerclient.Request) (runnerclient.Response, error) {
	return s.response, s.err
}

func TestDependencyErrorsAreMappedAndLogged(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		failAt     string
		runnerErr  error
		wantStatus int
		wantBody   string
		wantLog    string
	}{
		{name: "user upsert during submit", path: "submit", failAt: "upsert_user", wantStatus: http.StatusInternalServerError, wantBody: "could not save your submission", wantLog: "database sentinel failure"},
		{name: "submission save", path: "submit", failAt: "record_submission", wantStatus: http.StatusInternalServerError, wantBody: "could not save your submission", wantLog: "database sentinel failure"},
		{name: "completion save", path: "submit", failAt: "complete_lesson", wantStatus: http.StatusInternalServerError, wantBody: "could not save your submission", wantLog: "database sentinel failure"},
		{name: "user upsert during progress", path: "progress", failAt: "upsert_user", wantStatus: http.StatusInternalServerError, wantBody: "could not load your progress", wantLog: "database sentinel failure"},
		{name: "progress read", path: "progress", failAt: "read_progress", wantStatus: http.StatusInternalServerError, wantBody: "could not load your progress", wantLog: "database sentinel failure"},
		{name: "total XP read", path: "progress", failAt: "total_xp", wantStatus: http.StatusInternalServerError, wantBody: "could not load your progress", wantLog: "database sentinel failure"},
		{name: "runner failure", path: "submit", runnerErr: errors.New("runner sentinel failure"), wantStatus: http.StatusBadGateway, wantBody: "runner is unavailable", wantLog: "runner sentinel failure"},
		{name: "runner rate limited", path: "submit", runnerErr: runnerclient.ErrRateLimited, wantStatus: http.StatusServiceUnavailable, wantBody: "runner is busy", wantLog: "runner rate limited the request"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			previousLogger := slog.Default()
			var logOutput bytes.Buffer
			slog.SetDefault(slog.New(slog.NewTextHandler(&logOutput, nil)))
			defer slog.SetDefault(previousLogger)
			dataStore := newFailingTestStore(test.failAt)
			responseFromRunner := okResponse()
			if test.runnerErr != nil {
				responseFromRunner = runnerclient.Response{}
			}
			handler := NewHandler(loadFixtureCourses(t), dataStore, runnerStub{response: responseFromRunner, err: test.runnerErr}, true)
			path := "/api/me/progress"
			if test.path == "submit" {
				path = "/api/courses/go-course/lessons/start/hello/submit"
			}
			request := httptest.NewRequest(http.MethodGet, path, nil)
			if test.path == "submit" {
				request.Method = http.MethodPost
				request.Body = io.NopCloser(strings.NewReader(`{"files":{"main.go":"package main"}}`))
			}
			request.Header.Set("X-Dev-User-Email", "failure@example.test")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus || !strings.Contains(response.Body.String(), test.wantBody) {
				t.Fatalf("response = %d %s, want status %d containing %q", response.Code, response.Body.String(), test.wantStatus, test.wantBody)
			}
			if strings.Contains(response.Body.String(), test.wantLog) {
				t.Fatalf("response leaked underlying error: %s", response.Body.String())
			}
			requestID := response.Header().Get("X-Request-ID")
			if requestID == "" || !strings.Contains(logOutput.String(), requestID) || !strings.Contains(logOutput.String(), test.wantLog) || !strings.Contains(logOutput.String(), "level=ERROR") {
				t.Fatalf("error log = %q, want request ID, error %q, and error level", logOutput.String(), test.wantLog)
			}
			if test.failAt == "upsert_user" && strings.Contains(response.Body.String(), "authenticate") {
				t.Fatalf("database error was reported as authentication failure: %s", response.Body.String())
			}
		})
	}
}
