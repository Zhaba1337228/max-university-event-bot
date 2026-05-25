package main

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestRunReturnsUsageWithoutCommand(t *testing.T) {
	prevArgs := os.Args
	t.Cleanup(func() { os.Args = prevArgs })

	os.Args = []string{"migrate"}

	err := run()
	if !errors.Is(err, errUsage) {
		t.Fatalf("run() error = %v, want errUsage", err)
	}
}

func TestRunReturnsUsageForHelp(t *testing.T) {
	prevArgs := os.Args
	t.Cleanup(func() { os.Args = prevArgs })

	os.Args = []string{"migrate", "--help"}

	err := run()
	if !errors.Is(err, errUsage) {
		t.Fatalf("run() error = %v, want errUsage", err)
	}
}

func TestRunRequiresDatabaseURL(t *testing.T) {
	prevArgs := os.Args
	t.Cleanup(func() { os.Args = prevArgs })

	os.Args = []string{"migrate", "status"}
	t.Setenv("DATABASE_URL", "")

	err := run()
	if err == nil {
		t.Fatal("run() error = nil, want DATABASE_URL error")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL is required") {
		t.Fatalf("run() error = %v, want DATABASE_URL is required", err)
	}
}
