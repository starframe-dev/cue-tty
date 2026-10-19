package ansi

import (
	"github.com/rivo/uniseg"
	"github.com/starframe-dev/portalis"
	"strings"
)

// Cell is a single character cell on the terminal screen.
type Cell = portalis.Cell

// Cursor tracks the current terminal cursor position.
type Cursor = portalis.Cursor

// Screen is a virtual terminal screen backed by Portalis.
type Screen struct {
	*portalis.Screen
	Style Style
}

// EmptyCell returns a zero-value cell.
func EmptyCell() Cell { return Cell{} }

// NewScreen creates a screen with the given dimensions.
func NewScreen(cols, rows int) *Screen {
	return &Screen{Screen: portalis.NewScreen(rows, cols), Style: Reset()}
}

// Resize changes the screen size, preserving content where possible.
func (s *Screen) Resize(cols, rows int) {
	s.Screen.Resize(rows, cols)
}

// ClearScreen clears the active buffer.
func (s *Screen) ClearScreen() { s.Screen.Clear() }

// CurrentCursor returns a snapshot of the active cursor.
func (s *Screen) CurrentCursor() Cursor { return s.Screen.CursorState() }

// MoveCursor sets the cursor position (0-based).
func (s *Screen) MoveCursor(row, col int) { s.Screen.SetCursor(row, col) }

// MoveCursorRel moves the cursor relative to its current position.
func (s *Screen) MoveCursorRel(dy, dx int) {
	if dy < 0 {
		s.Screen.CursorUp(-dy)
	} else if dy > 0 {
		s.Screen.CursorDown(dy)
	}
	if dx < 0 {
		s.Screen.CursorBackward(-dx)
	} else if dx > 0 {
		s.Screen.CursorForward(dx)
	}
}

func (s *Screen) scrollUp(n int)   { s.Screen.ScrollRegionUp(n) }
func (s *Screen) scrollDown(n int) { s.Screen.ScrollRegionDown(n) }

// SwitchAltBuffer toggles between the main and alternate screen buffers.
func (s *Screen) SwitchAltBuffer(use bool) {
	if use {
		s.Screen.EnterAltScreen()
	} else {
		s.Screen.ExitAltScreen()
	}
}

// RenderText renders visible cells as plain text.
func (s *Screen) RenderText() string {
	cells := s.CellsSnapshot()
	var out strings.Builder
	for row, line := range cells {
		if row > 0 {
			out.WriteByte('\n')
		}
		for _, cell := range line {
			if cell.Continuation {
				continue
			}
			if cell.Rune == 0 {
				out.WriteByte(' ')
			} else {
				out.WriteRune(cell.Rune)
				out.WriteString(cell.Combining)
			}
		}
	}
	return out.String()
}

// CellsView returns a flat snapshot of the active visible buffer.
func (s *Screen) CellsView() []Cell {
	rows := s.CellsSnapshot()
	flat := make([]Cell, 0, s.Rows()*s.Cols())
	for _, row := range rows {
		flat = append(flat, row...)
	}
	return flat
}

// Current returns the active visible buffer as a flat snapshot.
func (s *Screen) Current() []Cell { return s.CellsView() }

func (s *Screen) cell(row, col int) *Cell {
	cell, ok := s.Screen.CellAt(row, col)
	if !ok {
		return nil
	}
	return &cell
}

func (s *Screen) clearWrapPending() { s.Screen.ClearWrapPending() }

// Put writes a rune with Portalis' current rendition.
func (s *Screen) Put(ch rune) { s.Screen.Put(ch) }

// StringWidth returns the display width of a string.
func StringWidth(text string) int { return uniseg.StringWidth(text) }
