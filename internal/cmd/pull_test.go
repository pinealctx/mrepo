package cmd

import (
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
