package validation

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/kinonoja/gobot/services/runner/internal/types"
)

const (
	MaxFiles         = 10
	MaxPayloadSize   = 100 * 1024
	MaxExpectedTests = 100
)

var expectedTestNamePattern = regexp.MustCompile(`^Test[A-Za-z0-9_]*$`)

func Validate(req *types.RunRequest) error {
	if req.Mode != "run" && req.Mode != "test" {
		return fmt.Errorf("invalid mode: %q", req.Mode)
	}

	if len(req.Files) == 0 {
		return fmt.Errorf("no files provided")
	}

	if req.TimeoutMs <= 0 {
		return fmt.Errorf("timeout_ms must be positive")
	}

	if len(req.GraderFiles) > 0 && req.Mode != "test" {
		return fmt.Errorf("grader_files are only allowed in test mode")
	}
	if len(req.ExpectedTests) > 0 && req.Mode != "test" {
		return fmt.Errorf("expected_tests are only allowed in test mode")
	}
	if len(req.ExpectedTests) > MaxExpectedTests {
		return fmt.Errorf("too many expected_tests: %d (max %d)", len(req.ExpectedTests), MaxExpectedTests)
	}

	if len(req.Files)+len(req.GraderFiles) > MaxFiles {
		return fmt.Errorf("too many files: %d (max %d)", len(req.Files)+len(req.GraderFiles), MaxFiles)
	}

	var totalSize int
	studentNames := sortedNames(req.Files)
	studentNamesByLower := make(map[string]string, len(studentNames))
	for _, name := range studentNames {
		if err := validateSafeFileName("student", name); err != nil {
			return err
		}
		if name == "go.mod" || name == "go.work" || name == "go.sum" {
			return fmt.Errorf("invalid student file name %q: reserved file name", name)
		}
		if strings.HasSuffix(name, "_test.go") {
			return fmt.Errorf("invalid student file name %q: student files cannot end in _test.go", name)
		}
		if !strings.HasSuffix(name, ".go") {
			return fmt.Errorf("invalid student file name %q: must end in .go", name)
		}
		lowerName := strings.ToLower(name)
		if prior, exists := studentNamesByLower[lowerName]; exists {
			return fmt.Errorf("student file names %q and %q collide after lower-casing", prior, name)
		}
		studentNamesByLower[lowerName] = name
		totalSize += len(req.Files[name])
	}

	graderNames := sortedNames(req.GraderFiles)
	for _, name := range graderNames {
		if err := validateSafeFileName("grader", name); err != nil {
			return err
		}
		if !strings.HasSuffix(name, "_test.go") {
			return fmt.Errorf("invalid grader file name %q: must end in _test.go", name)
		}
		if studentName, exists := studentNamesByLower[strings.ToLower(name)]; exists {
			return fmt.Errorf("grader file name %q collides with student file %q after lower-casing", name, studentName)
		}
		totalSize += len(req.GraderFiles[name])
	}

	seenTests := make(map[string]struct{}, len(req.ExpectedTests))
	for _, name := range req.ExpectedTests {
		if name == "" || !expectedTestNamePattern.MatchString(name) {
			return fmt.Errorf("invalid expected test name %q: must match ^Test[A-Za-z0-9_]*$", name)
		}
		if _, exists := seenTests[name]; exists {
			return fmt.Errorf("duplicate expected test name %q", name)
		}
		seenTests[name] = struct{}{}
	}

	if totalSize > MaxPayloadSize {
		return fmt.Errorf("total payload size %d exceeds limit %d", totalSize, MaxPayloadSize)
	}

	return nil
}

func validateSafeFileName(kind, name string) error {
	if name == "" {
		return fmt.Errorf("invalid %s file name %q: name must not be empty", kind, name)
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("invalid %s file name %q: contains \"..\"", kind, name)
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("invalid %s file name %q: contains path separator", kind, name)
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("invalid %s file name %q: name must not start with \".\"", kind, name)
	}
	return nil
}

func sortedNames(files map[string]string) []string {
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
