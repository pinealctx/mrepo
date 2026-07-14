package cmd

import (
	"errors"
	"strings"
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		// Short strings: no truncation.
		{"hello", 10, "hello"},
		{"hi", 5, "hi"},
		{"abcdef", 6, "abcdef"},
		// Needs truncation.
		{"abcdefg", 6, "abc..."},
		{"hello world", 8, "hello..."},
		// UTF-8 multibyte: rune-safe truncation.
		// "你好世界再见" = 6 runes, max=5, cut=2 → "你好..."
		{"你好世界再见", 5, "你好..."},
		// "abcdefghij" = 10 runes, max=7, cut=4 → "abcd..."
		{"abcdefghij", 7, "abcd..."},
		// max <= 3: just "...".
		{"abcdef", 3, "..."},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.max)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
		}
	}
}

func TestSingleLine(t *testing.T) {
	got := singleLine("Updating abc..def\r\nFast-forward\n 2 files changed")
	want := "Updating abc..def Fast-forward 2 files changed"
	if got != want {
		t.Fatalf("singleLine() = %q, want %q", got, want)
	}
}

func TestOperationOutputSummary(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		fallback string
		want     string
	}{
		{name: "empty", fallback: "up to date", want: "up to date"},
		{name: "up to date", output: "Already up to date.", want: "up to date"},
		{name: "fast forward", output: "Updating abc..def\nFast-forward\n2 files changed", want: "updated · fast-forward"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := operationOutputSummary(tt.output, tt.fallback); got != tt.want {
				t.Fatalf("operationOutputSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOperationErrorSummary(t *testing.T) {
	tests := []struct {
		err  error
		want string
	}{
		{errors.New("fetch: context deadline exceeded"), "timed out"},
		{errors.New("fetch: fatal: OpenSSL SSL_connect: SSL_ERROR_SYSCALL"), "network error · TLS connection failed"},
		{errors.New("merge: fatal: Not possible to fast-forward, aborting."), "Not possible to fast-forward, aborting."},
	}
	for _, tt := range tests {
		if got := operationErrorSummary(tt.err); got != tt.want {
			t.Errorf("operationErrorSummary(%q) = %q, want %q", tt.err, got, tt.want)
		}
	}
}

func TestRenderOperationTableKeepsRowsSingleLine(t *testing.T) {
	rows := []operationRow{
		{Icon: successIcon(), Name: "repo-a", Result: "Already clean", ResultStyle: dimStyle},
		{Icon: errorIcon(), Name: "repo-with-a-long-name", Result: "first line\nsecond line", ResultStyle: errorStyle},
	}
	rendered := renderOperationTable(rows)
	lines := strings.Split(rendered, "\n")
	if len(lines) != 4 {
		t.Fatalf("rendered table has %d lines, want 4:\n%s", len(lines), rendered)
	}
	if !strings.Contains(rendered, "first line second line") {
		t.Fatalf("rendered table did not collapse result to one line:\n%s", rendered)
	}
}
