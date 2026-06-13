package cue

import (
	"bytes"
	"testing"
)

func TestResolveKey(t *testing.T) {
	tests := []struct {
		key      string
		expected []byte
	}{
		{"Enter", []byte{0x0D}},
		{"Tab", []byte{0x09}},
		{"Up", []byte{0x1B, 0x5B, 0x41}},
		{"Down", []byte{0x1B, 0x5B, 0x42}},
		{"F1", []byte{0x1B, 0x4F, 0x50}},
		{"Ctrl+C", []byte{0x03}},
		{"Ctrl+A", []byte{0x01}},
		{"a", []byte{'a'}},
		{"X", []byte{'X'}},
	}

	for _, tt := range tests {
		got := ResolveKey(tt.key)
		if !bytes.Equal(got, tt.expected) {
			t.Errorf("ResolveKey(%q) = %v, want %v", tt.key, got, tt.expected)
		}
	}
}
