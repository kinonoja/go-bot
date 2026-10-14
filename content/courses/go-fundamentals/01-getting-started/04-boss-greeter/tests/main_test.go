package main

import "testing"

func TestGreetNamedPerson(t *testing.T) {
	if got := Greet("Ada"); got != "Hello, Ada!" {
		t.Fatalf("Greet(Ada) = %q, want %q", got, "Hello, Ada!")
	}
}

func TestGreetBlankName(t *testing.T) {
	if got := Greet("  "); got != "Hello, stranger!" {
		t.Fatalf("Greet(blank) = %q, want %q", got, "Hello, stranger!")
	}
}
