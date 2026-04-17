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
			name:     "empty string stays empty",
			input:    "",
			expected: "",
		},
		{
			name:     "all invalid characters returns empty",
			input:    "<>:\"/\\|?*" + "\x00\x01",
			expected: "",
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
