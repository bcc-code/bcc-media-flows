package cache

import (
	"testing"
	"time"
)

func TestGetTypeMismatchIsMiss(t *testing.T) {
	type a struct{ X int }
	type b struct{ Y string }

	Set("mismatch", &a{X: 1}, DefaultTTL)

	if got := Get[b]("mismatch"); got != nil {
		t.Fatalf("expected nil for mismatched type, got %v", got)
	}
	if got := Get[a]("mismatch"); got == nil || got.X != 1 {
		t.Fatalf("expected original value, got %v", got)
	}
}

func TestSetCustomTTLExpiry(t *testing.T) {
	v := "hello"
	Set("short", &v, 10*time.Millisecond)

	if got := Get[string]("short"); got == nil || *got != "hello" {
		t.Fatalf("expected value before expiry, got %v", got)
	}

	time.Sleep(20 * time.Millisecond)

	if got := Get[string]("short"); got != nil {
		t.Fatalf("expected nil after expiry, got %v", got)
	}
}
