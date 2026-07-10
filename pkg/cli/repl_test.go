package cli

import (
	"reflect"
	"testing"
)

func TestParseLine(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected []string
	}{
		{
			name:     "empty",
			line:     "",
			expected: nil,
		},
		{
			name:     "simple command",
			line:     "launch echo hello",
			expected: []string{"launch", "echo", "hello"},
		},
		{
			name:     "quoted argument",
			line:     `type "hello world"`,
			expected: []string{"type", "hello world"},
		},
		{
			name:     "mixed quoted and unquoted",
			line:     `click listitem:"  Item 2"`,
			expected: []string{"click", "listitem:  Item 2"},
		},
		{
			name:     "extra spaces",
			line:     "  launch   echo   hello  ",
			expected: []string{"launch", "echo", "hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseLine(tt.line)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("parseLine(%q) = %v, want %v", tt.line, got, tt.expected)
			}
		})
	}
}
