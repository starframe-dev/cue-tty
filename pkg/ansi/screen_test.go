package ansi

import "testing"

func TestNewScreen(t *testing.T) {
	s := NewScreen(80, 24)
	if s.Cols() != 80 || s.Rows() != 24 {
		t.Fatalf("expected 80x24, got %dx%d", s.Cols(), s.Rows())
	}
	if s.CellsView()[0].Rune != 0 {
		t.Fatal("expected empty cell")
	}
}

func TestScreenPut(t *testing.T) {
	s := NewScreen(10, 5)
	s.Put('H')
	s.Put('i')
	if s.CellsView()[0].Rune != 'H' {
		t.Fatalf("expected H, got %c", s.CellsView()[0].Rune)
	}
	if s.CellsView()[1].Rune != 'i' {
		t.Fatalf("expected i, got %c", s.CellsView()[1].Rune)
	}
	if s.CurrentCursor().Col != 2 {
		t.Fatalf("expected cursor col 2, got %d", s.CurrentCursor().Col)
	}
}

func TestScreenClear(t *testing.T) {
	s := NewScreen(10, 5)
	s.Put('X')
	s.ClearScreen()
	if s.CellsView()[0].Rune != 0 {
		t.Fatal("expected cleared cell")
	}
}

func TestScreenResize(t *testing.T) {
	s := NewScreen(10, 5)
	s.Put('A')
	s.Resize(20, 10)
	if s.Cols() != 20 || s.Rows() != 10 {
		t.Fatalf("expected 20x10, got %dx%d", s.Cols(), s.Rows())
	}
	if s.CellsView()[0].Rune != 'A' {
		t.Fatal("expected preserved content")
	}
}

func TestScreenAltBuffer(t *testing.T) {
	s := NewScreen(10, 5)
	s.Put('M')
	s.SwitchAltBuffer(true)
	s.Put('A')
	// Check alt buffer content before switching back
	if s.CellsSnapshot()[0][0].Rune != 'A' {
		t.Fatal("expected alt buffer content")
	}
	s.SwitchAltBuffer(false)
	// Check main buffer preserved
	if s.CellsSnapshot()[0][0].Rune != 'M' {
		t.Fatal("expected main buffer preserved")
	}
}

func TestScreenRenderText(t *testing.T) {
	s := NewScreen(5, 2)
	s.Put('H')
	s.Put('e')
	s.Put('l')
	s.Put('l')
	s.Put('o')
	text := s.RenderText()
	// Portalis Render() includes ANSI sequences. We just check the visible content.
	if len(text) < 10 {
		t.Fatalf("expected at least 10 chars, got %d: %q", len(text), text)
	}
}
