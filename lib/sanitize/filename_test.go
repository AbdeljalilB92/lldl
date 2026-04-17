package sanitize

import "testing"

func TestToSafeFileName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain text unchanged",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "strip angle brackets",
			input:    "file<name>",
			expected: "filename",
		},
		{
			name:     "strip colon and quotes",
			input:    `time: "now"`,
			expected: "time now",
		},
		{
			name:     "strip slashes and backslash",
			input:    `path/to\file`,
			expected: "pathtofile",
		},
		{
			name:     "strip pipe question and asterisk",
			input:    "what|is*this?",
			expected: "whatisthis",
		},
		{
			name:     "strip all special chars at once",
			input:    `A<>B:"C/D\E|F?G*H`,
			expected: "ABCDEFGH",
		},
		{
			name:     "preserve spaces",
			input:    "a b   c",
			expected: "a b   c",
		},
		{
			name:     "preserve unicode",
			input:    "Ca\u00f1\u00f3n del R\u00edo",
			expected: "Ca\u00f1\u00f3n del R\u00edo",
		},
		{
			name:     "preserve CJK characters",
			input:    "\u4e2d\u6587\u6807\u9898",
			expected: "\u4e2d\u6587\u6807\u9898",
		},
		{
			name:     "preserve emoji",
			input:    "React \U0001f60a Course",
			expected: "React \U0001f60a Course",
		},
		{
			name:     "empty string returns unnamed",
			input:    "",
			expected: "unnamed",
		},
		{
			name:     "all invalid characters returns unnamed",
			input:    "<>:\"/\\|?*" + "\x00\x01",
			expected: "unnamed",
		},
		{
			name:     "control characters stripped",
			input:    "line1" + "\x00" + "line2" + "\x1F" + "line3",
			expected: "line1line2line3",
		},
		{
			name:     "tab stripped (is a control character)",
			input:    "col1\tcol2",
			expected: "col1col2",
		},
		{
			name:     "newline stripped (control char)",
			input:    "line1\nline2",
			expected: "line1line2",
		},
		// Windows reserved name handling (#18)
		{
			name:     "Windows reserved name CON prefixed",
			input:    "CON",
			expected: "_CON",
		},
		{
			name:     "Windows reserved name PRN prefixed",
			input:    "PRN",
			expected: "_PRN",
		},
		{
			name:     "Windows reserved name AUX prefixed",
			input:    "AUX",
			expected: "_AUX",
		},
		{
			name:     "Windows reserved name NUL prefixed",
			input:    "NUL",
			expected: "_NUL",
		},
		{
			name:     "Windows reserved name COM1 prefixed",
			input:    "COM1",
			expected: "_COM1",
		},
		{
			name:     "Windows reserved name LPT9 prefixed",
			input:    "LPT9",
			expected: "_LPT9",
		},
		{
			name:     "Windows reserved name case-insensitive",
			input:    "con",
			expected: "_con",
		},
		{
			name:     "non-reserved name unchanged",
			input:    "Config",
			expected: "Config",
		},
		// Trailing spaces and periods (#19)
		{
			name:     "trailing spaces stripped",
			input:    "hello   ",
			expected: "hello",
		},
		{
			name:     "trailing periods stripped",
			input:    "hello...",
			expected: "hello",
		},
		{
			name:     "trailing spaces and periods stripped",
			input:    "hello .  ..",
			expected: "hello",
		},
		{
			name:     "only trailing stripped, internal preserved",
			input:    "hello. world  ",
			expected: "hello. world",
		},
		{
			name:     "only spaces becomes unnamed",
			input:    "   ",
			expected: "unnamed",
		},
		{
			name:     "only periods becomes unnamed",
			input:    "...",
			expected: "unnamed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToSafeFileName(tt.input)
			if got != tt.expected {
				t.Errorf("ToSafeFileName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
