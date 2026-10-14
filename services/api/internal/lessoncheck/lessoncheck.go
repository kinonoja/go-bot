package lessoncheck

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kinonoja/gobot/services/api/internal/lessons"
)

const RequestTimeoutMs = 30000

type RunnerRequest struct {
	Files         map[string]string `json:"files"`
	GraderFiles   map[string]string `json:"grader_files"`
	ExpectedTests []string          `json:"expected_tests"`
	Mode          string            `json:"mode"`
	TimeoutMs     int               `json:"timeout_ms"`
}

type RunnerTestResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
}

type RunnerResponse struct {
	Status       string             `json:"status"`
	Tests        []RunnerTestResult `json:"tests"`
	MissingTests []string           `json:"missing_tests"`
}

type Result struct {
	LessonPath      string
	SolutionPassed  bool
	StarterRejected bool
	SolutionDetail  string
	StarterDetail   string
}

func BuildRequest(files, graderFiles map[string]string, expectedTests []string) RunnerRequest {
	return RunnerRequest{
		Files:         cloneFiles(files),
		GraderFiles:   cloneFiles(graderFiles),
		ExpectedTests: append([]string(nil), expectedTests...),
		Mode:          "test",
		TimeoutMs:     RequestTimeoutMs,
	}
}

func EvaluateSolution(response RunnerResponse, expectedTests []string) error {
	if response.Status != "ok" {
		return fmt.Errorf("solution status is %q, want %q", response.Status, "ok")
	}
	if len(response.MissingTests) != 0 {
		return fmt.Errorf("solution missing tests: %v", response.MissingTests)
	}
	results := make(map[string]bool, len(response.Tests))
	for _, result := range response.Tests {
		results[result.Name] = result.Passed
	}
	for _, name := range expectedTests {
		passed, exists := results[name]
		if !exists {
			return fmt.Errorf("solution has no result for expected test %q", name)
		}
		if !passed {
			return fmt.Errorf("solution test %q failed", name)
		}
	}
	return nil
}

func EvaluateStarter(response RunnerResponse) error {
	if response.Status == "compile_error" {
		return nil
	}
	if response.Status != "runtime_error" {
		return fmt.Errorf("starter status is %q, want compile_error or failing tests", response.Status)
	}
	for _, result := range response.Tests {
		if !result.Passed {
			return nil
		}
	}
	return fmt.Errorf("starter did not produce a failing expected test or compile error")
}

func CheckLesson(ctx context.Context, client *http.Client, runnerURL string, course lessons.Course, chapter lessons.Chapter, lesson lessons.Lesson) Result {
	result := Result{
		LessonPath: fmt.Sprintf("%s/%s/%s", course.Slug, chapter.Slug, lesson.Slug),
	}
	graderFiles := lesson.GraderFilesForInternalUse()
	expectedTests := lesson.ExpectedTests

	solutionRequest := BuildRequest(lesson.SolutionFilesForInternalUse(), graderFiles, expectedTests)
	solutionResponse, err := sendRequest(ctx, client, runnerURL, solutionRequest)
	if err != nil {
		result.SolutionDetail = err.Error()
	} else if err := EvaluateSolution(solutionResponse, expectedTests); err != nil {
		result.SolutionDetail = err.Error()
	} else {
		result.SolutionPassed = true
		result.SolutionDetail = "all expected tests passed"
	}

	starterRequest := BuildRequest(lesson.StarterFiles, graderFiles, expectedTests)
	starterResponse, err := sendRequest(ctx, client, runnerURL, starterRequest)
	if err != nil {
		result.StarterDetail = err.Error()
	} else if err := EvaluateStarter(starterResponse); err != nil {
		result.StarterDetail = err.Error()
	} else {
		result.StarterRejected = true
		result.StarterDetail = "starter fails as expected"
	}
	return result
}

func sendRequest(ctx context.Context, client *http.Client, runnerURL string, payload RunnerRequest) (RunnerResponse, error) {
	if client == nil {
		client = http.DefaultClient
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return RunnerResponse{}, fmt.Errorf("encode runner request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(runnerURL, "/")+"/run", bytes.NewReader(body))
	if err != nil {
		return RunnerResponse{}, fmt.Errorf("create runner request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return RunnerResponse{}, fmt.Errorf("call runner: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return RunnerResponse{}, fmt.Errorf("runner returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	var result RunnerResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return RunnerResponse{}, fmt.Errorf("decode runner response: %w", err)
	}
	return result, nil
}

func cloneFiles(files map[string]string) map[string]string {
	copyOfFiles := make(map[string]string, len(files))
	for name, content := range files {
		copyOfFiles[name] = content
	}
	return copyOfFiles
}
