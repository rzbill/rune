package watcher

import (
	"testing"
	"time"
)

func TestFormatAge(t *testing.T) {
	if got := formatAge(time.Time{}); got != "Unknown" {
		t.Fatalf("formatAge zero: got %q", got)
	}
	if got := formatAge(time.Now().Add(-10 * time.Second)); got != "Just now" {
		t.Fatalf("formatAge <1m: got %q", got)
	}
	if got := formatAge(time.Now().Add(-2 * time.Minute)); got != "2m" {
		t.Fatalf("formatAge minutes: got %q", got)
	}
	if got := formatAge(time.Now().Add(-3 * time.Hour)); got != "3h" {
		t.Fatalf("formatAge hours: got %q", got)
	}
	if got := formatAge(time.Now().Add(-2 * 24 * time.Hour)); got != "2d" {
		t.Fatalf("formatAge days: got %q", got)
	}
	if got := formatAge(time.Now().Add(-3 * 30 * 24 * time.Hour)); got != "3mo" {
		t.Fatalf("formatAge months: got %q", got)
	}
	if got := formatAge(time.Now().Add(-2 * 365 * 24 * time.Hour)); got != "2y" {
		t.Fatalf("formatAge years: got %q", got)
	}
}

func TestTruncateString(t *testing.T) {
	if got := truncateString("short", 10); got != "short" {
		t.Fatalf("unexpected truncate for short string: %q", got)
	}
	if got := truncateString("longstring", 5); got != "lo..." {
		t.Fatalf("unexpected truncate: %q", got)
	}
}
