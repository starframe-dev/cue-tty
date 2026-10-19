package ansi

import (
	"testing"

	"github.com/starframe-dev/portalis"
)

func cellAt(s *Screen, r, c int) Cell {
	cell, ok := s.Screen.CellAt(r, c)
	if !ok {
		return Cell{}
	}
	return cell
}

func TestParserCaretIsPrinted(t *testing.T) {
	s := NewScreen(80, 24)
	p := NewParser(s)
	p.FeedString("a^b")
	if cellAt(s, 0, 0).Rune != 'a' || cellAt(s, 0, 1).Rune != '^' || cellAt(s, 0, 2).Rune != 'b' {
		t.Fatalf("expected a^b to be printed, got %q", s.RenderText())
	}
}

func TestParserInvalidSGRPaletteIndicesDoNotPanic(t *testing.T) {
	for _, sequence := range []string{
		"\x1b[38;5;256mX",
		"\x1b[48;5;999999999999999999999999mX",
		"\x1b[38;2;999;-1;1mX",
	} {
		t.Run(sequence, func(t *testing.T) {
			s := NewScreen(80, 24)
			p := NewParser(s)
			p.FeedString(sequence)
			if cellAt(s, 0, 0).Rune != 'X' {
				t.Fatalf("parser lost printable text after malformed SGR %q", sequence)
			}
		})
	}
}

func FuzzParserFeed(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte("a^b"),
		[]byte("\x1b[38;5;256mX"),
		[]byte("^[[6n"),
		[]byte("\x1b]0;title\x07"),
		[]byte("\xe2\x95\xad"),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		screen := NewScreen(80, 24)
		parser := NewParser(screen)
		for start := 0; start < len(data); {
			end := start + 17
			if end > len(data) {
				end = len(data)
			}
			parser.Feed(data[start:end])
			start = end
		}
	})
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
	// ANSI index 196 is standard bright red in the 6x6x6 cube.
	if cellAt(s, 0, 0).FG != "#ff0000" {
		t.Fatalf("expected fg #ff0000, got %q", cellAt(s, 0, 0).FG)
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

func TestParserDSRRespondsWithCurrentCursorAcrossChunks(t *testing.T) {
	screen := NewScreen(80, 24)
	parser := NewParser(screen)
	var response []byte
	parser.SetResponseCallback(func(data []byte) { response = append(response, data...) })
	parser.FeedString("abc\x1b[")
	parser.FeedString("6n")
	if got, want := string(response), "\x1b[1;4R"; got != want {
		t.Fatalf("DSR response = %q, want %q", got, want)
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
	if cellAt(s, 0, 0).FG != "#ff0000" {
		t.Fatalf("expected fg #ff0000, got %q", cellAt(s, 0, 0).FG)
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
