package main

import "testing"

func TestTriplePositive(t *testing.T) {
	if got := Triple(4); got != 12 {
		t.Fatalf("Triple(4) = %d, want 12", got)
	}
}

func TestTripleZero(t *testing.T) {
	if got := Triple(0); got != 0 {
		t.Fatalf("Triple(0) = %d, want 0", got)
	}
}
