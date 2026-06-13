package ansi

import "testing"

func TestNewScreen(t *testing.T) {
    s := NewScreen(80, 24)
    if s.Cols != 80 || s.Rows != 24 {
        t.Fatalf("expected 80x24, got %dx%d", s.Cols, s.Rows)
    }
    if s.CellsView()[0].Rune != ' ' {
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
    if s.Cursor.Col != 2 {
        t.Fatalf("expected cursor col 2, got %d", s.Cursor.Col)
    }
}

func TestScreenClear(t *testing.T) {
    s := NewScreen(10, 5)
    s.Put('X')
    s.ClearScreen()
    if s.CellsView()[0].Rune != ' ' {
        t.Fatal("expected cleared cell")
    }
}

func TestScreenResize(t *testing.T) {
    s := NewScreen(10, 5)
    s.Put('A')
    s.Resize(20, 10)
    if s.Cols != 20 || s.Rows != 10 {
        t.Fatalf("expected 20x10, got %dx%d", s.Cols, s.Rows)
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
    if s.altCells[0].Rune != 'A' {
        t.Fatal("expected alt buffer content")
    }
    if s.cells[0].Rune != 'M' {
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
    expected := "Hello\n     "
    if text != expected {
        t.Fatalf("expected %q, got %q", expected, text)
    }
}
