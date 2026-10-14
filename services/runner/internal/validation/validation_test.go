package validation

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kinonoja/gobot/services/runner/internal/types"
)

func TestValidate(t *testing.T) {
	validRequest := func(files map[string]string, mode string) types.RunRequest {
		return types.RunRequest{Files: files, Mode: mode, TimeoutMs: 5000}
	}
	tooManyCombinedFiles := make(map[string]string, MaxFiles)
	for i := 0; i < MaxFiles; i++ {
		tooManyCombinedFiles[fmt.Sprintf("file%d.go", i)] = "x"
	}
	tooManyExpectedTests := make([]string, MaxExpectedTests+1)
	for i := range tooManyExpectedTests {
		tooManyExpectedTests[i] = fmt.Sprintf("Test%d", i)
	}

	tests := []struct {
		name    string
		req     types.RunRequest
		wantErr string
	}{
		{
			name:    "valid run request using files only",
			req:     validRequest(map[string]string{"main.go": "package main"}, "run"),
			wantErr: "",
		},
		{
			name: "valid test request with grader files and expected tests",
			req: func() types.RunRequest {
				req := validRequest(map[string]string{"main.go": "package main"}, "test")
				req.GraderFiles = map[string]string{"grader_test.go": "package main"}
				req.ExpectedTests = []string{"TestOne", "Test_two9"}
				return req
			}(),
			wantErr: "",
		},
		{name: "invalid mode", req: types.RunRequest{Files: map[string]string{"main.go": "x"}, Mode: "exec", TimeoutMs: 5000}, wantErr: `invalid mode: "exec"`},
		{name: "no student files", req: validRequest(map[string]string{}, "run"), wantErr: "no files provided"},
		{name: "zero timeout", req: types.RunRequest{Files: map[string]string{"main.go": "x"}, Mode: "run"}, wantErr: "timeout_ms must be positive"},
		{name: "negative timeout", req: types.RunRequest{Files: map[string]string{"main.go": "x"}, Mode: "run", TimeoutMs: -1}, wantErr: "timeout_ms must be positive"},
		{name: "empty student file name", req: validRequest(map[string]string{"": "x"}, "run"), wantErr: `invalid student file name "": name must not be empty`},
		{name: "student file without go extension", req: validRequest(map[string]string{"main.txt": "x"}, "run"), wantErr: `invalid student file name "main.txt": must end in .go`},
		{name: "student test file rejected", req: validRequest(map[string]string{"main_test.go": "x"}, "test"), wantErr: `invalid student file name "main_test.go": student files cannot end in _test.go`},
		{name: "student go.mod rejected", req: validRequest(map[string]string{"go.mod": "x"}, "run"), wantErr: `invalid student file name "go.mod": reserved file name`},
		{name: "student go.work rejected", req: validRequest(map[string]string{"go.work": "x"}, "run"), wantErr: `invalid student file name "go.work": reserved file name`},
		{name: "student go.sum rejected", req: validRequest(map[string]string{"go.sum": "x"}, "run"), wantErr: `invalid student file name "go.sum": reserved file name`},
		{name: "student path traversal rejected", req: validRequest(map[string]string{"../evil.go": "x"}, "run"), wantErr: `invalid student file name "../evil.go": contains ".."`},
		{name: "student forward slash rejected", req: validRequest(map[string]string{"sub/main.go": "x"}, "run"), wantErr: `invalid student file name "sub/main.go": contains path separator`},
		{name: "student backslash rejected", req: validRequest(map[string]string{"sub\\main.go": "x"}, "run"), wantErr: `invalid student file name "sub\\main.go": contains path separator`},
		{name: "dot-prefixed student name rejected", req: validRequest(map[string]string{".hidden.go": "x"}, "run"), wantErr: `invalid student file name ".hidden.go": name must not start with "."`},
		{name: "student names collide after lower-casing", req: validRequest(map[string]string{"Main.go": "x", "main.go": "y"}, "run"), wantErr: `student file names "Main.go" and "main.go" collide after lower-casing`},
		{
			name: "grader files rejected in run mode",
			req: func() types.RunRequest {
				req := validRequest(map[string]string{"main.go": "x"}, "run")
				req.GraderFiles = map[string]string{"grader_test.go": "x"}
				return req
			}(),
			wantErr: "grader_files are only allowed in test mode",
		},
		{name: "empty grader file name", req: func() types.RunRequest {
			req := validRequest(map[string]string{"main.go": "x"}, "test")
			req.GraderFiles = map[string]string{"": "x"}
			return req
		}(), wantErr: `invalid grader file name "": name must not be empty`},
		{name: "grader file with wrong suffix", req: func() types.RunRequest {
			req := validRequest(map[string]string{"main.go": "x"}, "test")
			req.GraderFiles = map[string]string{"grader.go": "x"}
			return req
		}(), wantErr: `invalid grader file name "grader.go": must end in _test.go`},
		{name: "grader path traversal rejected", req: func() types.RunRequest {
			req := validRequest(map[string]string{"main.go": "x"}, "test")
			req.GraderFiles = map[string]string{"../grader_test.go": "x"}
			return req
		}(), wantErr: `invalid grader file name "../grader_test.go": contains ".."`},
		{name: "grader path separator rejected", req: func() types.RunRequest {
			req := validRequest(map[string]string{"main.go": "x"}, "test")
			req.GraderFiles = map[string]string{"sub/grader_test.go": "x"}
			return req
		}(), wantErr: `invalid grader file name "sub/grader_test.go": contains path separator`},
		{name: "dot-prefixed grader name rejected", req: func() types.RunRequest {
			req := validRequest(map[string]string{"main.go": "x"}, "test")
			req.GraderFiles = map[string]string{".grader_test.go": "x"}
			return req
		}(), wantErr: `invalid grader file name ".grader_test.go": name must not start with "."`},
		{name: "grader collides with student after lower-casing", req: func() types.RunRequest {
			req := validRequest(map[string]string{"grader_TEST.go": "x"}, "test")
			req.GraderFiles = map[string]string{"grader_test.go": "y"}
			return req
		}(), wantErr: `grader file name "grader_test.go" collides with student file "grader_TEST.go" after lower-casing`},
		{
			name: "expected tests rejected in run mode",
			req: func() types.RunRequest {
				req := validRequest(map[string]string{"main.go": "x"}, "run")
				req.ExpectedTests = []string{"TestOne"}
				return req
			}(),
			wantErr: "expected_tests are only allowed in test mode",
		},
		{name: "invalid expected test name", req: func() types.RunRequest {
			req := validRequest(map[string]string{"main.go": "x"}, "test")
			req.ExpectedTests = []string{"Test-Bad"}
			return req
		}(), wantErr: `invalid expected test name "Test-Bad": must match ^Test[A-Za-z0-9_]*$`},
		{name: "empty expected test name", req: func() types.RunRequest {
			req := validRequest(map[string]string{"main.go": "x"}, "test")
			req.ExpectedTests = []string{""}
			return req
		}(), wantErr: `invalid expected test name "": must match ^Test[A-Za-z0-9_]*$`},
		{name: "duplicate expected test names", req: func() types.RunRequest {
			req := validRequest(map[string]string{"main.go": "x"}, "test")
			req.ExpectedTests = []string{"TestOne", "TestOne"}
			return req
		}(), wantErr: `duplicate expected test name "TestOne"`},
		{name: "too many expected tests", req: func() types.RunRequest {
			req := validRequest(map[string]string{"main.go": "x"}, "test")
			req.ExpectedTests = tooManyExpectedTests
			return req
		}(), wantErr: fmt.Sprintf("too many expected_tests: %d (max %d)", MaxExpectedTests+1, MaxExpectedTests)},
		{name: "combined file count too large", req: func() types.RunRequest {
			req := validRequest(tooManyCombinedFiles, "test")
			req.GraderFiles = map[string]string{"grader_test.go": "x"}
			return req
		}(), wantErr: fmt.Sprintf("too many files: %d (max %d)", MaxFiles+1, MaxFiles)},
		{name: "combined payload too large", req: func() types.RunRequest {
			req := validRequest(map[string]string{"main.go": strings.Repeat("x", MaxPayloadSize/2+1)}, "test")
			req.GraderFiles = map[string]string{"grader_test.go": strings.Repeat("y", MaxPayloadSize/2)}
			return req
		}(), wantErr: fmt.Sprintf("total payload size %d exceeds limit %d", MaxPayloadSize+1, MaxPayloadSize)},
		{name: "payload at exact limit", req: validRequest(map[string]string{"main.go": strings.Repeat("x", MaxPayloadSize)}, "run"), wantErr: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(&tt.req)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %q, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() error = nil, want %q", tt.wantErr)
			}
			if err.Error() != tt.wantErr {
				t.Fatalf("Validate() error = %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}
