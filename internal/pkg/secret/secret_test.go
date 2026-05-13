package secret_test

import (
	"strings"
	"testing"

	"github.com/Zhaba1337228/max-university-event-bot/internal/pkg/secret"
)

func TestMask(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"x", "***"},
		{"short", "***"},
		{"abcdefghijkl", "abcd***ijkl"},     // ровно 12
		{"abcdefghijklmnop", "abcd***mnop"}, // длиннее
		{"REAL_LONG_BOT_TOKEN_HERE", "REAL***HERE"},
	}
	for _, tc := range cases {
		got := secret.Mask(tc.in)
		if got != tc.want {
			t.Errorf("Mask(%q) = %q, want %q", tc.in, got, tc.want)
		}
		if strings.Contains(got, tc.in) && len(tc.in) >= 12 {
			t.Errorf("Mask(%q) leaks original: %q", tc.in, got)
		}
	}
}

func TestMaskHeader(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"": "",
		"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9": "Bearer eyJh***VCJ9",
		"Basic dXNlcjpwYXNzd29yZA==":                  "Basic dXNl***ZA==",
		"Token abcd1234efgh5678":                      "Token abcd***5678",
		"plain-secret-without-prefix":                 "plai***efix",
	}
	for in, want := range cases {
		got := secret.MaskHeader(in)
		if got != want {
			t.Errorf("MaskHeader(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestConstantTimeEqual(t *testing.T) {
	t.Parallel()

	if !secret.ConstantTimeEqual("hello", "hello") {
		t.Error("equal strings must compare true")
	}
	if secret.ConstantTimeEqual("hello", "world") {
		t.Error("different strings must compare false")
	}
	if secret.ConstantTimeEqual("hello", "hello-world") {
		t.Error("different length must compare false")
	}
	if secret.ConstantTimeEqual("", "") != true {
		t.Error("empty strings must compare true")
	}
}
