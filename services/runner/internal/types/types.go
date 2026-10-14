package types

type RunRequest struct {
	Files         map[string]string `json:"files"`
	GraderFiles   map[string]string `json:"grader_files,omitempty"`
	ExpectedTests []string          `json:"expected_tests,omitempty"`
	Mode          string            `json:"mode"`
	TimeoutMs     int               `json:"timeout_ms"`
}

type TestResult struct {
	Name      string `json:"name"`
	Passed    bool   `json:"passed"`
	Output    string `json:"output"`
	Completed bool   `json:"-"`
}

type RunResponse struct {
	Status       string       `json:"status"`
	Stdout       string       `json:"stdout"`
	Stderr       string       `json:"stderr"`
	DurationMs   int64        `json:"duration_ms"`
	Tests        []TestResult `json:"tests,omitempty"`
	MissingTests []string     `json:"missing_tests"`
}
