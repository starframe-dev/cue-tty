package ansi

import (
	"testing"

	"github.com/Starframe/portalis"
)

func cellAt(s *Screen, r, c int) *Cell {
	return &s.Cells[r][c]
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
	if cellAt(s, 0, 0).Rune != 0 {
		t.Fatalf("expected cleared cell, got %c", cellAt(s, 0, 0).Rune)
	}
}

func TestParserColors(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)
	p.FeedString("\x1b[31mRed\x1b[0m")
	// Red is ANSI color 1 → "#800000"
	if cellAt(s, 0, 0).FG != "#800000" {
		t.Fatalf("expected red fg #800000, got %q", cellAt(s, 0, 0).FG)
	}
}

func TestParserBold(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)
	p.FeedString("\x1b[1mBold\x1b[22m")
	if cellAt(s, 0, 0).Style&portalis.StyleBold == 0 {
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
	p.FeedString("Main")
	p.FeedString("\x1b[?1049hAlt\x1b[?1049l")
	// After switching back to main buffer, main should have "Main"
	if cellAt(s, 0, 0).Rune != 'M' {
		t.Fatalf("expected M in main buffer, got %c", cellAt(s, 0, 0).Rune)
	}
	if cellAt(s, 0, 1).Rune != 'a' {
		t.Fatalf("expected a in main buffer, got %c", cellAt(s, 0, 1).Rune)
	}
}

func TestParserSGR256Color(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)
	p.FeedString("\x1b[38;5;196mRed256\x1b[0m")
	// Color 196 is a 6x6x6 cube color: n=196, n-16=180, r=180/36=5, g=(180/6)%6=30%6=0, b=180%6=0
	// r=5*42+12=222, g=0*42+12=12, b=0*42+12=12 → #de0c0c
	if cellAt(s, 0, 0).FG != "#de0c0c" {
		t.Fatalf("expected fg #de0c0c, got %q", cellAt(s, 0, 0).FG)
	}
	if cellAt(s, 0, 0).Rune != 'R' {
		t.Fatalf("expected R, got %c", cellAt(s, 0, 0).Rune)
	}
}

func TestParserSGRTrueColor(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)
	p.FeedString("\x1b[38;2;255;128;64mOrange\x1b[0m")
	if cellAt(s, 0, 0).FG != "#ff8040" {
		t.Fatalf("expected fg #ff8040, got %q", cellAt(s, 0, 0).FG)
	}
	if cellAt(s, 0, 0).Rune != 'O' {
		t.Fatalf("expected O, got %c", cellAt(s, 0, 0).Rune)
	}
}

func TestParserSGRTrueColorBackground(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)
	p.FeedString("\x1b[48;2;40;40;40mBg\x1b[0m")
	if cellAt(s, 0, 0).BG != "#282828" {
		t.Fatalf("expected bg #282828, got %q", cellAt(s, 0, 0).BG)
	}
}

func TestParserSGRCombinedTrueColor(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)
	// Foreground and background true-color in one SGR sequence.
	p.FeedString("\x1b[38;2;251;241;199;48;2;40;40;40mX\x1b[0m")
	if cellAt(s, 0, 0).FG != "#fbf1c7" {
		t.Fatalf("expected fg #fbf1c7, got %q", cellAt(s, 0, 0).FG)
	}
	if cellAt(s, 0, 0).BG != "#282828" {
		t.Fatalf("expected bg #282828, got %q", cellAt(s, 0, 0).BG)
	}
	if cellAt(s, 0, 0).Rune != 'X' {
		t.Fatalf("expected X, got %c", cellAt(s, 0, 0).Rune)
	}
}

func TestParserSplitEscapeSequence(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)
	seq := "\x1b[38;5;196mRed\x1b[0m"
	splitAt := 6 // middle of the SGR sequence
	p.FeedString(seq[:splitAt])
	p.FeedString(seq[splitAt:])
	if cellAt(s, 0, 0).Rune != 'R' {
		t.Fatalf("expected R, got %c", cellAt(s, 0, 0).Rune)
	}
	if cellAt(s, 0, 0).FG != "#de0c0c" {
		t.Fatalf("expected fg #de0c0c, got %q", cellAt(s, 0, 0).FG)
	}
	if cellAt(s, 0, 3).FG != "" {
		t.Fatalf("expected reset fg after Red, got %q", cellAt(s, 0, 3).FG)
	}
}

func TestParserSplitUTF8(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)
	p.FeedString("\xe2")
	p.FeedString("\x95\xad")
	if cellAt(s, 0, 0).Rune != '╭' {
		t.Fatalf("expected ╭, got %c", cellAt(s, 0, 0).Rune)
	}
}

func TestParserModalTitleLine(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)
	line := "\x1b[48;2;60;56;54m\x1b[38;2;215;153;33m│\x1b[0m\x1b[48;2;60;56;54m\x1b[38;2;215;153;33m\x1b[1m  Folder name\x1b[0m\x1b[48;2;60;56;54m      \x1b[0m\x1b[48;2;60;56;54m\x1b[38;2;204;36;29m✕\x1b[0m\x1b[48;2;60;56;54m\x1b[38;2;215;153;33m  │\x1b[0m"
	p.FeedString(line)
	if cellAt(s, 0, 0).Rune != '│' {
		t.Fatalf("expected │ at col 0, got %q", cellAt(s, 0, 0).Rune)
	}
	if cellAt(s, 0, 20).Rune != '✕' {
		t.Fatalf("expected ✕ at col 20, got %q", cellAt(s, 0, 20).Rune)
	}
	if cellAt(s, 0, 23).Rune != '│' {
		t.Fatalf("expected │ at col 23, got %q", cellAt(s, 0, 23).Rune)
	}
}

func TestParserModalTitleLineSplit(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)
	line := "\x1b[48;2;60;56;54m\x1b[38;2;215;153;33m│\x1b[0m\x1b[48;2;60;56;54m\x1b[38;2;215;153;33m\x1b[1m  Folder name\x1b[0m\x1b[48;2;60;56;54m      \x1b[0m\x1b[48;2;60;56;54m\x1b[38;2;204;36;29m✕\x1b[0m\x1b[48;2;60;56;54m\x1b[38;2;215;153;33m  │\x1b[0m"
	// Feed byte-by-byte to simulate chunked PTY reads.
	for _, b := range []byte(line) {
		p.Feed([]byte{b})
	}
	if cellAt(s, 0, 0).Rune != '│' {
		t.Fatalf("expected │ at col 0, got %q", cellAt(s, 0, 0).Rune)
	}
	if cellAt(s, 0, 20).Rune != '✕' {
		t.Fatalf("expected ✕ at col 20, got %q", cellAt(s, 0, 20).Rune)
	}
	if cellAt(s, 0, 23).Rune != '│' {
		t.Fatalf("expected │ at col 23, got %q", cellAt(s, 0, 23).Rune)
	}
}
