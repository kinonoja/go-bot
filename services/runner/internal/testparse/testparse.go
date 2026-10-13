package testparse

import (
	"bufio"
	"encoding/json"
	"strings"

	"github.com/kinonoja/gobot/services/runner/internal/types"
)

type TestEvent struct {
	Action  string `json:"Action"`
	Package string `json:"Package"`
	Test    string `json:"Test"`
	Output  string `json:"Output"`
}

func Parse(jsonOutput string) []types.TestResult {
	results := make(map[string]*types.TestResult)
	var order []string

	scanner := bufio.NewScanner(strings.NewReader(jsonOutput))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var event TestEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		if event.Test == "" {
			continue
		}

		key := event.Package + "/" + event.Test
		if _, exists := results[key]; !exists {
			results[key] = &types.TestResult{Name: event.Test}
			order = append(order, key)
		}

		r := results[key]

		switch event.Action {
		case "output":
			r.Output += event.Output
		case "pass":
			r.Passed = true
		case "fail":
			r.Passed = false
		}
	}

	out := make([]types.TestResult, 0, len(order))
	for _, key := range order {
		out = append(out, *results[key])
	}
	return out
}
