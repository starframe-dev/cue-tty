package ansi

import (
	"fmt"

	"github.com/Starframe/portalis"
	"github.com/charmbracelet/lipgloss"
	"github.com/rivo/uniseg"
)

// Cell is a single character on the screen.
type Cell = portalis.Cell

// Cursor tracks the current cursor position.
type Cursor = portalis.Cursor

// EmptyCell returns a zero-value cell (empty).
func EmptyCell() Cell { return Cell{} }

// Screen is a virtual terminal screen backed by portalis.Screen.
type Screen struct {
	*portalis.Screen
	Style Style
}

// NewScreen creates a screen with the given dimensions.
func NewScreen(cols, rows int) *Screen {
	return &Screen{Screen: portalis.NewScreen(rows, cols), Style: Reset()}
}

// Resize changes the screen size, preserving top-left content.
func (s *Screen) Resize(cols, rows int) {
	s.Screen.Resize(rows, cols)
}

// ClearScreen clears the entire active buffer.
func (s *Screen) ClearScreen() {
	s.Screen.Clear()
}

// CurrentCursor returns the active cursor.
func (s *Screen) CurrentCursor() *Cursor {
	return &s.Screen.Cursor
}

// MoveCursor sets the cursor position (0-based, clamped).
func (s *Screen) MoveCursor(row, col int) {
	s.Screen.SetCursor(row, col)
}

// MoveCursorRel moves the cursor relative to current position.
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

// scrollUp scrolls the scroll region up by n lines.
func (s *Screen) scrollUp(n int) {
	s.Screen.ScrollRegionUp(n)
}

// scrollDown scrolls the scroll region down by n lines.
func (s *Screen) scrollDown(n int) {
	s.Screen.ScrollRegionDown(n)
}

// SwitchAltBuffer toggles between main and alternate screen buffer.
func (s *Screen) SwitchAltBuffer(use bool) {
	if use {
		s.Screen.EnterAltScreen()
	} else {
		s.Screen.ExitAltScreen()
	}
}

// RenderText renders the screen as plain text (no ANSI sequences).
func (s *Screen) RenderText() string {
	return stripANSI(s.Screen.Render())
}

// stripANSI removes ANSI escape sequences from a string.
func stripANSI(s string) string {
	var out []byte
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' {
			// Skip escape sequence
			i++
			if i < len(s) && s[i] == '[' {
				i++
				for i < len(s) && !(s[i] >= 0x40 && s[i] <= 0x7E) {
					i++
				}
				if i < len(s) {
					i++
				}
			} else if i < len(s) {
				// Two-byte escape
				i++
			}
		} else {
			out = append(out, s[i])
			i++
		}
	}
	return string(out)
}

// CellsView returns a flat slice of the active buffer for inspection.
func (s *Screen) CellsView() []Cell {
	flat := make([]Cell, s.Rows*s.Cols)
	for r := 0; r < s.Rows; r++ {
		copy(flat[r*s.Cols:(r+1)*s.Cols], s.Cells[r])
	}
	return flat
}

// Current returns the active buffer as a flat slice.
func (s *Screen) Current() []Cell {
	return s.CellsView()
}

// cell returns a pointer to the cell at (r, c).
func (s *Screen) cell(r, c int) *Cell {
	return &s.Screen.Cells[r][c]
}

// clearWrapPending resets the wrap-pending flag.
func (s *Screen) clearWrapPending() {
	s.Screen.ClearWrapPending()
}

// Put writes a rune at the current cursor with the current style.
// Handles wide characters (CJK, emoji) by advancing cursor by display width.
func (s *Screen) Put(ch rune) {
	s.syncStyle()
	s.Screen.Put(ch)
}

// syncStyle copies the wrapper Style to the Portalis cursor so Put applies it.
func (s *Screen) syncStyle() {
	st := &s.Screen.Cursor
	st.Style = 0
	if s.Style.Bold {
		st.Style |= portalis.StyleBold
	}
	if s.Style.Dim {
		st.Style |= portalis.StyleDim
	}
	if s.Style.Italic {
		st.Style |= portalis.StyleItalic
	}
	if s.Style.Underline {
		st.Style |= portalis.StyleUnderline
	}
	if s.Style.Reverse {
		st.Style |= portalis.StyleReverse
	}
	st.FG = colorToLipgloss(s.Style.Fg)
	st.BG = colorToLipgloss(s.Style.Bg)
}

// colorToLipgloss converts an ansi.Color to a lipgloss.Color string.
func colorToLipgloss(c Color) lipgloss.Color {
	if c.Is256 {
		return ansi256Color(int(c.Index))
	}
	return lipgloss.Color(rgb(int(c.R), int(c.G), int(c.B)))
}

// ansi256Color converts an ANSI color index (0-255) to a lipgloss.Color string.
func ansi256Color(n int) lipgloss.Color {
	if n < 0 || n > 255 {
		return ""
	}
	if n < 16 {
		colors := []string{
			"#000000", "#800000", "#008000", "#808000",
			"#000080", "#800080", "#008080", "#c0c0c0",
			"#808080", "#ff0000", "#00ff00", "#ffff00",
			"#0000ff", "#ff00ff", "#00ffff", "#ffffff",
		}
		return lipgloss.Color(colors[n])
	}
	if n < 232 {
		n -= 16
		r := n / 36
		g := (n / 6) % 6
		b := n % 6
		return lipgloss.Color(rgb(r*42+12, g*42+12, b*42+12))
	}
	v := (n-232)*10 + 8
	return lipgloss.Color(rgb(v, v, v))
}

func rgb(r, g, b int) string {
	return "#" + hexByte(r) + hexByte(g) + hexByte(b)
}

func hexByte(n int) string {
	return fmt.Sprintf("%02x", n)
}

// StringWidth returns the display width of a string.
func StringWidth(s string) int {
	return uniseg.StringWidth(s)
}
