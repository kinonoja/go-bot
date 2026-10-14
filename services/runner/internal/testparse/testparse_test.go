package testparse

import (
	"testing"

	"github.com/kinonoja/gobot/services/runner/internal/types"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantCount  int
		wantNames  []string
		wantPassed []bool
	}{
		{
			name:       "empty input",
			input:      "",
			wantCount:  0,
			wantNames:  nil,
			wantPassed: nil,
		},
		{
			name: "single passing test",
			input: `{"Action":"run","Package":"submission","Test":"TestAdd"}
{"Action":"output","Package":"submission","Test":"TestAdd","Output":"=== RUN   TestAdd\n"}
{"Action":"output","Package":"submission","Test":"TestAdd","Output":"--- PASS: TestAdd (0.00s)\n"}
{"Action":"pass","Package":"submission","Test":"TestAdd"}
`,
			wantCount:  1,
			wantNames:  []string{"TestAdd"},
			wantPassed: []bool{true},
		},
		{
			name: "single failing test",
			input: `{"Action":"run","Package":"submission","Test":"TestAdd"}
{"Action":"output","Package":"submission","Test":"TestAdd","Output":"=== RUN   TestAdd\n"}
{"Action":"output","Package":"submission","Test":"TestAdd","Output":"    add_test.go:10: expected 4, got 5\n"}
{"Action":"fail","Package":"submission","Test":"TestAdd"}
`,
			wantCount:  1,
			wantNames:  []string{"TestAdd"},
			wantPassed: []bool{false},
		},
		{
			name: "mixed pass and fail",
			input: `{"Action":"run","Package":"submission","Test":"TestAdd"}
{"Action":"pass","Package":"submission","Test":"TestAdd"}
{"Action":"run","Package":"submission","Test":"TestSub"}
{"Action":"fail","Package":"submission","Test":"TestSub"}
`,
			wantCount:  2,
			wantNames:  []string{"TestAdd", "TestSub"},
			wantPassed: []bool{true, false},
		},
		{
			name: "skips package-level events",
			input: `{"Action":"output","Package":"submission","Output":"ok  \tsubmission\t0.002s\n"}
{"Action":"pass","Package":"submission"}
`,
			wantCount:  0,
			wantNames:  nil,
			wantPassed: nil,
		},
		{
			name: "handles invalid json lines",
			input: `not json at all
{"Action":"run","Package":"submission","Test":"TestOK"}
{"Action":"pass","Package":"submission","Test":"TestOK"}
`,
			wantCount:  1,
			wantNames:  []string{"TestOK"},
			wantPassed: []bool{true},
		},
		{
			name: "accumulates output",
			input: `{"Action":"run","Package":"submission","Test":"TestLog"}
{"Action":"output","Package":"submission","Test":"TestLog","Output":"line1\n"}
{"Action":"output","Package":"submission","Test":"TestLog","Output":"line2\n"}
{"Action":"pass","Package":"submission","Test":"TestLog"}
`,
			wantCount:  1,
			wantNames:  []string{"TestLog"},
			wantPassed: []bool{true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := Parse(tt.input)
			if len(results) != tt.wantCount {
				t.Fatalf("expected %d results, got %d", tt.wantCount, len(results))
			}
			for i, r := range results {
				if r.Name != tt.wantNames[i] {
					t.Errorf("result[%d] name = %q, want %q", i, r.Name, tt.wantNames[i])
				}
				if r.Passed != tt.wantPassed[i] {
					t.Errorf("result[%d] passed = %v, want %v", i, r.Passed, tt.wantPassed[i])
				}
			}
		})
	}
}

func TestParseOutputAccumulation(t *testing.T) {
	input := `{"Action":"run","Package":"submission","Test":"TestLog"}
{"Action":"output","Package":"submission","Test":"TestLog","Output":"hello "}
{"Action":"output","Package":"submission","Test":"TestLog","Output":"world\n"}
{"Action":"pass","Package":"submission","Test":"TestLog"}
`
	results := Parse(input)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	want := "hello world\n"
	if results[0].Output != want {
		t.Errorf("output = %q, want %q", results[0].Output, want)
	}
}

func TestMissing(t *testing.T) {
	tests := []struct {
		name     string
		expected []string
		results  []types.TestResult
		want     []string
	}{
		{
			name:     "all expected tests have pass or fail outcomes",
			expected: []string{"TestPass", "TestFail"},
			results: []types.TestResult{
				{Name: "TestPass", Passed: true, Completed: true},
				{Name: "TestFail", Passed: false, Completed: true},
			},
			want: []string{},
		},
		{
			name:     "some expected tests are missing or incomplete",
			expected: []string{"TestPass", "TestMissing", "TestIncomplete"},
			results: []types.TestResult{
				{Name: "TestPass", Passed: true, Completed: true},
				{Name: "TestIncomplete", Completed: false},
			},
			want: []string{"TestMissing", "TestIncomplete"},
		},
		{
			name:     "none of the expected tests have results",
			expected: []string{"TestOne", "TestTwo"},
			results:  []types.TestResult{},
			want:     []string{"TestOne", "TestTwo"},
		},
		{
			name:     "empty expected list",
			expected: []string{},
			results:  []types.TestResult{{Name: "TestOne", Passed: true, Completed: true}},
			want:     []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Missing(tt.expected, tt.results)
			if len(got) != len(tt.want) {
				t.Fatalf("Missing() = %#v, want %#v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Fatalf("Missing() = %#v, want %#v", got, tt.want)
				}
			}
			if len(tt.want) == 0 && got == nil {
				t.Fatal("Missing() returned nil, want empty list")
			}
		})
	}
}
