package main

import "testing"

func TestGreeting(t *testing.T) {
	if got := Greeting(); got != "Hello, Go!" {
		t.Fatalf("Greeting() = %q, want %q", got, "Hello, Go!")
	}
}

func TestGreetingIsStable(t *testing.T) {
	first := Greeting()
	second := Greeting()
	if first != second {
		t.Fatalf("Greeting() changed from %q to %q", first, second)
	}
	if first != "Hello, Go!" {
		t.Fatalf("Greeting() = %q, want %q", first, "Hello, Go!")
	}
}
