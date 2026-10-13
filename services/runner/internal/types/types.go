package types

type RunRequest struct {
	Files     map[string]string `json:"files"`
	Mode      string            `json:"mode"`
	TimeoutMs int               `json:"timeout_ms"`
}

type TestResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Output string `json:"output"`
}

type RunResponse struct {
	Status     string       `json:"status"`
	Stdout     string       `json:"stdout"`
	Stderr     string       `json:"stderr"`
	DurationMs int64        `json:"duration_ms"`
	Tests      []TestResult `json:"tests,omitempty"`
}
