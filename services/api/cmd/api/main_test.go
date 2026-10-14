package main

import "testing"

func TestEnvOr(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fallback string
		want     string
	}{
		{name: "uses value", value: "8090", fallback: "8081", want: "8090"},
		{name: "uses fallback for empty value", value: "", fallback: "8081", want: "8081"},
		{name: "uses fallback for whitespace", value: "  ", fallback: "8081", want: "8081"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("GOBOT_API_TEST_ENV", test.value)
			if got := envOr("GOBOT_API_TEST_ENV", test.fallback); got != test.want {
				t.Fatalf("envOr() = %q, want %q", got, test.want)
			}
		})
	}
}
