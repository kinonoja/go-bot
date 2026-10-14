package main

import (
	"testing"

	"github.com/kinonoja/gobot/services/api/internal/lessoncheck"
)

func TestResultLabels(t *testing.T) {
	tests := []struct {
		name         string
		result       lessoncheck.Result
		wantSolution string
		wantStarter  string
	}{
		{
			name:         "both checks pass",
			result:       lessoncheck.Result{SolutionPassed: true, StarterRejected: true},
			wantSolution: "PASS",
			wantStarter:  "REJECTED",
		},
		{
			name:         "solution failed and starter passed",
			result:       lessoncheck.Result{},
			wantSolution: "FAIL",
			wantStarter:  "PASS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			solution, starter := resultLabels(tt.result)
			if solution != tt.wantSolution || starter != tt.wantStarter {
				t.Fatalf("resultLabels() = %q/%q, want %q/%q", solution, starter, tt.wantSolution, tt.wantStarter)
			}
		})
	}
}
