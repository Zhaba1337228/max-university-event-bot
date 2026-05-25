package main

import (
	"testing"
)

func TestRuntimeArchUsesGoEnvVars(t *testing.T) {
	t.Setenv("GOOS", "linux")
	t.Setenv("GOARCH", "amd64")

	if got := runtimeArch(); got != "linux/amd64" {
		t.Fatalf("runtimeArch() = %q, want %q", got, "linux/amd64")
	}
}
