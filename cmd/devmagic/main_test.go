package main

import (
	"os"
	"strings"
	"testing"
)

func TestRunRequiresSingleUserIDArgument(t *testing.T) {
	prevArgs := os.Args
	t.Cleanup(func() { os.Args = prevArgs })

	os.Args = []string{"devmagic"}

	err := run()
	if err == nil {
		t.Fatal("run() error = nil, want usage error")
	}
	if !strings.Contains(err.Error(), "usage: devmagic <user_id>") {
		t.Fatalf("run() error = %v, want usage message", err)
	}
}

func TestRunRejectsBadUserID(t *testing.T) {
	prevArgs := os.Args
	t.Cleanup(func() { os.Args = prevArgs })

	os.Args = []string{"devmagic", "not-a-number"}

	err := run()
	if err == nil {
		t.Fatal("run() error = nil, want parse error")
	}
	if !strings.Contains(err.Error(), "parse user_id") {
		t.Fatalf("run() error = %v, want parse user_id", err)
	}
}

func TestRunRequiresLongSessionKey(t *testing.T) {
	prevArgs := os.Args
	t.Cleanup(func() { os.Args = prevArgs })

	os.Args = []string{"devmagic", "42"}
	t.Setenv("ADMIN_SESSION_KEY", "short")
	t.Setenv("DATABASE_URL", "postgres://example")

	err := run()
	if err == nil {
		t.Fatal("run() error = nil, want ADMIN_SESSION_KEY validation error")
	}
	if !strings.Contains(err.Error(), "ADMIN_SESSION_KEY must be set") {
		t.Fatalf("run() error = %v, want ADMIN_SESSION_KEY validation", err)
	}
}

func TestRunRequiresDatabaseURL(t *testing.T) {
	prevArgs := os.Args
	t.Cleanup(func() { os.Args = prevArgs })

	os.Args = []string{"devmagic", "42"}
	t.Setenv("ADMIN_SESSION_KEY", strings.Repeat("x", 32))
	t.Setenv("DATABASE_URL", "")

	err := run()
	if err == nil {
		t.Fatal("run() error = nil, want DATABASE_URL error")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL must be set") {
		t.Fatalf("run() error = %v, want DATABASE_URL validation", err)
	}
}
