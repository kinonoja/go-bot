package validation

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kinonoja/gobot/services/runner/internal/types"
)

const (
	MaxFiles       = 10
	MaxPayloadSize = 100 * 1024
)

func Validate(req *types.RunRequest) error {
	if req.Mode != "run" && req.Mode != "test" {
		return fmt.Errorf("invalid mode: %q", req.Mode)
	}

	if len(req.Files) == 0 {
		return fmt.Errorf("no files provided")
	}

	if len(req.Files) > MaxFiles {
		return fmt.Errorf("too many files: %d (max %d)", len(req.Files), MaxFiles)
	}

	if req.TimeoutMs <= 0 {
		return fmt.Errorf("timeout_ms must be positive")
	}

	var totalSize int
	for name, content := range req.Files {
		if err := validateFileName(name); err != nil {
			return err
		}
		totalSize += len(content)
	}

	if totalSize > MaxPayloadSize {
		return fmt.Errorf("total payload size %d exceeds limit %d", totalSize, MaxPayloadSize)
	}

	return nil
}

func validateFileName(name string) error {
	if name == "" {
		return fmt.Errorf("empty file name")
	}

	if strings.Contains(name, "..") {
		return fmt.Errorf("invalid file name: %q contains \"..\"", name)
	}

	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("invalid file name: %q contains path separator", name)
	}

	if filepath.IsAbs(name) {
		return fmt.Errorf("invalid file name: %q is absolute path", name)
	}

	return nil
}
