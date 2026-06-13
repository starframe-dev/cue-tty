package ansi

import "testing"

func cellAt(s *Screen, r, c int) *Cell {
    return &s.CellsView()[r*s.Cols+c]
}

func TestParserPlainText(t *testing.T) {
    s := NewScreen(80, 24)
    p := NewParser(s)
    p.FeedString("Hello")
    if cellAt(s, 0, 0).Rune != 'H' {
        t.Fatalf("expected H, got %c", cellAt(s, 0, 0).Rune)
    }
    if cellAt(s, 0, 4).Rune != 'o' {
        t.Fatalf("expected o, got %c", cellAt(s, 0, 4).Rune)
    }
}

func TestParserCursorMovement(t *testing.T) {
    s := NewScreen(80, 24)
    p := NewParser(s)
    p.FeedString("AB\x1b[2;3HC")
    if cellAt(s, 1, 2).Rune != 'C' {
        t.Fatalf("expected C at (1,2), got %c", cellAt(s, 1, 2).Rune)
    }
}

func TestParserClearScreen(t *testing.T) {
    s := NewScreen(80, 24)
    p := NewParser(s)
    p.FeedString("Hello\x1b[2J")
    if cellAt(s, 0, 0).Rune != ' ' {
        t.Fatalf("expected cleared cell, got %c", cellAt(s, 0, 0).Rune)
    }
}

func TestParserColors(t *testing.T) {
    s := NewScreen(80, 24)
    p := NewParser(s)
    p.FeedString("\x1b[31mRed\x1b[0m")
    if cellAt(s, 0, 0).Style.Fg.Index != 1 {
        t.Fatalf("expected red fg, got %v", cellAt(s, 0, 0).Style.Fg)
    }
}

func TestParserBold(t *testing.T) {
    s := NewScreen(80, 24)
    p := NewParser(s)
    p.FeedString("\x1b[1mBold\x1b[22m")
    if !cellAt(s, 0, 0).Style.Bold {
        t.Fatal("expected bold")
    }
}

func TestParserNewline(t *testing.T) {
    s := NewScreen(80, 24)
    p := NewParser(s)
    p.FeedString("A\nB")
    if cellAt(s, 0, 0).Rune != 'A' {
        t.Fatal("expected A on line 0")
    }
    // LF moves down but does not reset column, so B is at (1,1)
    if cellAt(s, 1, 1).Rune != 'B' {
        t.Fatalf("expected B at (1,1), got %c", cellAt(s, 1, 1).Rune)
    }
}

func TestParserCarriageReturn(t *testing.T) {
    s := NewScreen(80, 24)
    p := NewParser(s)
    p.FeedString("ABC\rD")
    if cellAt(s, 0, 0).Rune != 'D' {
        t.Fatalf("expected D at col 0, got %c", cellAt(s, 0, 0).Rune)
    }
    if cellAt(s, 0, 1).Rune != 'B' {
        t.Fatalf("expected B at col 1, got %c", cellAt(s, 0, 1).Rune)
    }
}

func TestParserAltBuffer(t *testing.T) {
    s := NewScreen(80, 24)
    p := NewParser(s)
    p.FeedString("\x1b[?1049hAlt\x1b[?1049l")
    if s.altCells[0].Rune != 'A' {
        t.Fatalf("expected A in alt buffer, got %c", s.altCells[0].Rune)
    }
}
