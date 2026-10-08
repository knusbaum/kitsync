package kitsync

import (
	"testing"
)

func TestCleanStringBasic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"plain", "hello", "hello"},
		{"whitespace left", "  hello", "hello"},
		{"whitespace right", "hello  ", "hello"},
		{"whitespace both", "  hello  ", "hello"},
		// Newlines are valid UTF-8; ToValidUTF8 leaves them, TrimSpace trims nothing inside
		{"newline", "hello\nworld", "hello\nworld"},
		{"tab", "hello\tworld", "hello\tworld"},
		{"null byte", "hello\x00world", "hello?world"},
		{"unicode valid", "café", "café"},
		// \x80\xff is a maximal invalid sequence → single ? replacement
		{"unicode invalid", "café\x80\xffbar", "café?bar"},
		{"only spaces", "   ", ""},
		{"only tabs", "\t\t", ""},
		{"trailing null", "hello\x00", "hello?"},
		{"leading null", "\x00hello", "?hello"},
		{"multiple nulls", "a\x00b\x00c", "a?b?c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanString(tt.input)
			if got != tt.expected {
				t.Errorf("CleanString(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
