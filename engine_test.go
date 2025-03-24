package mold

import (
	"strconv"
	"testing"
)

func TestValidExt(t *testing.T) {
	tests := []struct {
		exts     []string
		ext      string
		expected bool
	}{
		{
			exts:     []string{".txt", ".go", ".html"},
			ext:      ".txt",
			expected: true,
		},
		{
			exts:     []string{".txt", ".go", ".html"},
			ext:      "txt",
			expected: true,
		},
		{
			exts:     []string{"txt", "go", "html"},
			ext:      ".txt",
			expected: true,
		},
		{
			exts:     []string{"txt", "go", "html"},
			ext:      "txt",
			expected: true,
		},
		{
			exts:     []string{".TXT", ".Go", ".HTML"},
			ext:      ".txt",
			expected: true,
		},
		{
			exts:     []string{".txt", ".go", ".html"},
			ext:      ".js",
			expected: false,
		},
		{
			exts:     []string{".txt", ".go", ".html"},
			ext:      "js",
			expected: false,
		},
		{
			exts:     []string{},
			ext:      ".txt",
			expected: false,
		},
		{
			exts:     []string{".txt", ".go", ".html"},
			ext:      "",
			expected: false,
		},
		{
			exts:     []string{".txt", ".go", ".html"},
			ext:      ".",
			expected: false,
		},
		{
			exts:     []string{".txt", ".go", ".html"},
			ext:      "...",
			expected: false,
		},
		{
			exts:     []string{".txt", ".go", ".html"},
			ext:      ".txt.js",
			expected: false,
		},
		{
			exts:     []string{".txt", ".go", ".html"},
			ext:      "txt.js",
			expected: false,
		},
		{
			exts:     []string{".txt", ".go", ".html"},
			ext:      ".txt.",
			expected: false,
		},
		{
			exts:     []string{".txt", ".go", ".html"},
			ext:      "txt.",
			expected: false,
		},
		{
			exts:     []string{".txt", ".go", ".html", ".tXt"},
			ext:      ".tXt",
			expected: true,
		},
		{
			exts:     []string{".txt", "go", ".html", ".tXt"},
			ext:      "tXt",
			expected: true,
		},
	}

	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			result := hasExt(tt.exts, tt.ext)
			if result != tt.expected {
				t.Errorf("validExt(%v, %q) = %v, expected %v", tt.exts, tt.ext, result, tt.expected)
			}
		})
	}
}
