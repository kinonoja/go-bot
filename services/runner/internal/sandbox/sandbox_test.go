package sandbox

import (
	"strings"
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLen   int
		wantTrunc bool
	}{
		{
			name:      "short string unchanged",
			input:     "hello",
			wantLen:   5,
			wantTrunc: false,
		},
		{
			name:      "exact limit unchanged",
			input:     strings.Repeat("x", MaxOutputBytes),
			wantLen:   MaxOutputBytes,
			wantTrunc: false,
		},
		{
			name:      "over limit gets truncated",
			input:     strings.Repeat("x", MaxOutputBytes+100),
			wantLen:   MaxOutputBytes + len(TruncationMsg),
			wantTrunc: true,
		},
		{
			name:      "empty string",
			input:     "",
			wantLen:   0,
			wantTrunc: false,
		},
		{
			name:      "one byte over limit",
			input:     strings.Repeat("x", MaxOutputBytes+1),
			wantLen:   MaxOutputBytes + len(TruncationMsg),
			wantTrunc: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Truncate(tt.input)
			if len(result) != tt.wantLen {
				t.Errorf("len = %d, want %d", len(result), tt.wantLen)
			}
			hasTrunc := strings.HasSuffix(result, TruncationMsg)
			if hasTrunc != tt.wantTrunc {
				t.Errorf("truncated = %v, want %v", hasTrunc, tt.wantTrunc)
			}
		})
	}
}
