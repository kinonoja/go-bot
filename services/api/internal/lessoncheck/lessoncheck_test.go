package lessoncheck

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kinonoja/gobot/services/api/internal/lessons"
)

func TestBuildRequest(t *testing.T) {
	tests := []struct {
		name          string
		files         map[string]string
		graderFiles   map[string]string
		expectedTests []string
	}{
		{
			name:          "creates test request with student and grader data",
			files:         map[string]string{"main.go": "package main"},
			graderFiles:   map[string]string{"grader_test.go": "package main"},
			expectedTests: []string{"TestOne", "TestTwo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildRequest(tt.files, tt.graderFiles, tt.expectedTests)
			if got.Mode != "test" || got.TimeoutMs != RequestTimeoutMs {
				t.Fatalf("mode/timeout = %q/%d", got.Mode, got.TimeoutMs)
			}
			if !reflect.DeepEqual(got.Files, tt.files) || !reflect.DeepEqual(got.GraderFiles, tt.graderFiles) || !reflect.DeepEqual(got.ExpectedTests, tt.expectedTests) {
				t.Fatalf("BuildRequest() = %#v", got)
			}
			got.Files["main.go"] = "changed"
			if tt.files["main.go"] == "changed" {
				t.Fatal("BuildRequest() reused the student file map")
			}
		})
	}
}

func TestEvaluateSolution(t *testing.T) {
	tests := []struct {
		name     string
		response RunnerResponse
		wantErr  bool
	}{
		{
			name: "all expected tests passed",
			response: RunnerResponse{
				Status:       "ok",
				Tests:        []RunnerTestResult{{Name: "TestOne", Passed: true}, {Name: "TestTwo", Passed: true}},
				MissingTests: []string{},
			},
		},
		{name: "non-ok status", response: RunnerResponse{Status: "compile_error"}, wantErr: true},
		{name: "missing expected result", response: RunnerResponse{Status: "ok", Tests: []RunnerTestResult{{Name: "TestOne", Passed: true}}, MissingTests: []string{}}, wantErr: true},
		{name: "missing tests reported", response: RunnerResponse{Status: "ok", Tests: []RunnerTestResult{{Name: "TestOne", Passed: true}, {Name: "TestTwo", Passed: true}}, MissingTests: []string{"TestTwo"}}, wantErr: true},
		{name: "expected test failed", response: RunnerResponse{Status: "ok", Tests: []RunnerTestResult{{Name: "TestOne", Passed: true}, {Name: "TestTwo", Passed: false}}, MissingTests: []string{}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EvaluateSolution(tt.response, []string{"TestOne", "TestTwo"})
			if (err != nil) != tt.wantErr {
				t.Fatalf("EvaluateSolution() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEvaluateStarter(t *testing.T) {
	tests := []struct {
		name     string
		response RunnerResponse
		wantErr  bool
	}{
		{name: "compile error", response: RunnerResponse{Status: "compile_error"}},
		{name: "expected test fails", response: RunnerResponse{Status: "runtime_error", Tests: []RunnerTestResult{{Name: "TestOne", Passed: false}}}},
		{name: "all tests pass", response: RunnerResponse{Status: "ok", Tests: []RunnerTestResult{{Name: "TestOne", Passed: true}, {Name: "TestTwo", Passed: true}}, MissingTests: []string{}}, wantErr: true},
		{name: "runtime error without expected test failure", response: RunnerResponse{Status: "runtime_error", Tests: []RunnerTestResult{{Name: "TestOne", Passed: true}}}, wantErr: true},
		{name: "timeout", response: RunnerResponse{Status: "timeout"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := EvaluateStarter(tt.response)
			if (err != nil) != tt.wantErr {
				t.Fatalf("EvaluateStarter() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCheckLessonWithFakeRunner(t *testing.T) {
	lessonSet := makeLessonFixture(t)
	var requests []RunnerRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/run" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		var request RunnerRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		requests = append(requests, request)
		response := RunnerResponse{
			Status:       "ok",
			Tests:        []RunnerTestResult{{Name: "TestOne", Passed: true}, {Name: "TestTwo", Passed: true}},
			MissingTests: []string{},
		}
		if request.Files["main.go"] == "starter" {
			response.Status = "runtime_error"
			response.Tests[1].Passed = false
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer server.Close()

	result := CheckLesson(context.Background(), server.Client(), server.URL, lessonSet.course, lessonSet.chapter, lessonSet.lesson)
	if !result.SolutionPassed || !result.StarterRejected {
		t.Fatalf("CheckLesson() = %#v", result)
	}
	if len(requests) != 2 {
		t.Fatalf("runner requests = %d, want 2", len(requests))
	}
	if requests[0].Files["main.go"] != "solution" || requests[1].Files["main.go"] != "starter" {
		t.Fatalf("request order/files = %#v", requests)
	}
	for _, request := range requests {
		if request.Mode != "test" || request.TimeoutMs != RequestTimeoutMs || len(request.GraderFiles) != 1 || !reflect.DeepEqual(request.ExpectedTests, []string{"TestOne", "TestTwo"}) {
			t.Fatalf("runner request fields = %#v", request)
		}
	}
}

type lessonFixture struct {
	course  lessons.Course
	chapter lessons.Chapter
	lesson  lessons.Lesson
}

func makeLessonFixture(t *testing.T) lessonFixture {
	t.Helper()
	root := t.TempDir()
	lessonRoot := filepath.Join(root, "course", "01-chapter", "01-lesson")
	writeTestFile(t, filepath.Join(root, "course", "course.yaml"), "title: Course\ndescription: Description\n")
	writeTestFile(t, filepath.Join(root, "course", "01-chapter", "chapter.yaml"), "title: Chapter\ndescription: Description\n")
	writeTestFile(t, filepath.Join(lessonRoot, "lesson.md"), "---\ntitle: Lesson\nxp: 20\ndifficulty: easy\nexpected_tests:\n  - TestOne\n  - TestTwo\n---\nGoal\n")
	writeTestFile(t, filepath.Join(lessonRoot, "starter", "main.go"), "starter")
	writeTestFile(t, filepath.Join(lessonRoot, "solution", "main.go"), "solution")
	writeTestFile(t, filepath.Join(lessonRoot, "tests", "main_test.go"), "package main\nimport \"testing\"\nfunc TestOne(t *testing.T) {}\nfunc TestTwo(t *testing.T) {}\n")
	courses, err := lessons.Load(root)
	if err != nil {
		t.Fatalf("lessons.Load() error = %v", err)
	}
	return lessonFixture{course: courses[0], chapter: courses[0].Chapters[0], lesson: courses[0].Chapters[0].Lessons[0]}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
