package cue

import (
	"bytes"
	"testing"
)

func TestMouseSGRCoordinatesExceeding999(t *testing.T) {
	got := string(MouseSGR(0, 1234, 1000, false))
	if got != "\x1b[<0;1235;1001M" {
		t.Fatalf("unexpected encoding for large coordinates: %q", got)
	}
}

func TestMouseSGRUsesRealButtonCodesForPressAndRelease(t *testing.T) {
	for button := 0; button <= 2; button++ {
		t.Run(string(rune('0'+button)), func(t *testing.T) {
			press := string(MouseSGR(button, 12, 34, false))
			wantPress := "\x1b[<" + string(rune('0'+button)) + ";13;35M"
			if press != wantPress {
				t.Fatalf("press = %q, want %q", press, wantPress)
			}
			release := string(MouseSGR(button, 12, 34, true))
			wantRelease := "\x1b[<" + string(rune('0'+button)) + ";13;35m"
			if release != wantRelease {
				t.Fatalf("release = %q, want %q", release, wantRelease)
			}
		})
	}
}

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
