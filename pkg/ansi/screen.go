package ansi

import "bytes"

// Screen is a virtual terminal screen backed by a flat cell slice.
type Screen struct {
    Cols         int
    Rows         int
    cells        []Cell
    altCells     []Cell
    Cursor       Cursor
    AltCursor    Cursor
    UseAltBuffer bool
    ScrollTop    int
    ScrollBottom int
    Style        Style
}

// Cursor tracks the current cursor position.
type Cursor struct {
    Row int
    Col int
}

// NewScreen creates a screen with the given dimensions.
func NewScreen(cols, rows int) *Screen {
    n := cols * rows
    s := &Screen{
        Cols:         cols,
        Rows:         rows,
        cells:        make([]Cell, n),
        altCells:     make([]Cell, n),
        ScrollTop:    0,
        ScrollBottom: rows - 1,
        Style:        Reset(),
    }
    for i := range s.cells {
        s.cells[i] = EmptyCell()
    }
    for i := range s.altCells {
        s.altCells[i] = EmptyCell()
    }
    return s
}

func (s *Screen) cell(r, c int) *Cell {
    return &s.current()[r*s.Cols+c]
}

// Current returns the active buffer as a flat slice.
func (s *Screen) Current() []Cell {
    if s.UseAltBuffer {
        return s.altCells
    }
    return s.cells
}

func (s *Screen) current() []Cell {
    return s.Current()
}

// Resize changes the screen size, preserving top-left content.
func (s *Screen) Resize(cols, rows int) {
    oldCells := s.current()
    oldCols, oldRows := s.Cols, s.Rows

    newN := cols * rows
    newCells := make([]Cell, newN)
    for i := range newCells {
        newCells[i] = EmptyCell()
    }
    for r := 0; r < min(rows, oldRows); r++ {
        for c := 0; c < min(cols, oldCols); c++ {
            newCells[r*cols+c] = oldCells[r*oldCols+c]
        }
    }

    if s.UseAltBuffer {
        s.altCells = newCells
        s.cells = make([]Cell, newN)
        for i := range s.cells {
            s.cells[i] = EmptyCell()
        }
    } else {
        s.cells = newCells
        s.altCells = make([]Cell, newN)
        for i := range s.altCells {
            s.altCells[i] = EmptyCell()
        }
    }

    s.Cols = cols
    s.Rows = rows
    s.ScrollBottom = rows - 1
    if s.Cursor.Row >= rows {
        s.Cursor.Row = rows - 1
    }
    if s.Cursor.Col >= cols {
        s.Cursor.Col = cols - 1
    }
}

// CurrentCursor returns the active cursor.
func (s *Screen) CurrentCursor() *Cursor {
    if s.UseAltBuffer {
        return &s.AltCursor
    }
    return &s.Cursor
}

// Put writes a rune at the current cursor with the current style.
func (s *Screen) Put(ch rune) {
    cur := s.CurrentCursor()
    if cur.Row < 0 || cur.Row >= s.Rows || cur.Col < 0 || cur.Col >= s.Cols {
        return
    }
    s.cell(cur.Row, cur.Col).Rune = ch
    s.cell(cur.Row, cur.Col).Style = s.Style
    cur.Col++
    if cur.Col >= s.Cols {
        cur.Col = 0
        cur.Row++
        if cur.Row > s.ScrollBottom {
            s.scrollUp(1)
            cur.Row = s.ScrollBottom
        }
    }
}

// MoveCursor sets the cursor position (0-based, clamped).
func (s *Screen) MoveCursor(row, col int) {
    cur := s.CurrentCursor()
    cur.Row = clamp(row, 0, s.Rows-1)
    cur.Col = clamp(col, 0, s.Cols-1)
}

// MoveCursorRel moves the cursor relative to current position.
func (s *Screen) MoveCursorRel(dy, dx int) {
    cur := s.CurrentCursor()
    s.MoveCursor(cur.Row+dy, cur.Col+dx)
}

// ClearScreen clears the entire active buffer.
func (s *Screen) ClearScreen() {
    cells := s.current()
    for i := range cells {
        cells[i] = EmptyCell()
    }
}

// ClearLine clears the current line from cursor to end.
func (s *Screen) ClearLine() {
    cur := s.CurrentCursor()
    if cur.Row < 0 || cur.Row >= s.Rows {
        return
    }
    start := cur.Row * s.Cols
    for i := start + cur.Col; i < start+s.Cols; i++ {
        s.current()[i] = EmptyCell()
    }
}

// ClearLineLeft clears from start of line to cursor.
func (s *Screen) ClearLineLeft() {
    cur := s.CurrentCursor()
    if cur.Row < 0 || cur.Row >= s.Rows {
        return
    }
    start := cur.Row * s.Cols
    for i := start; i <= start+cur.Col; i++ {
        s.current()[i] = EmptyCell()
    }
}

// ClearLineAll clears the entire current line.
func (s *Screen) ClearLineAll() {
    cur := s.CurrentCursor()
    if cur.Row < 0 || cur.Row >= s.Rows {
        return
    }
    start := cur.Row * s.Cols
    for i := start; i < start+s.Cols; i++ {
        s.current()[i] = EmptyCell()
    }
}

// ScrollUp scrolls the scroll region up by n lines.
func (s *Screen) scrollUp(n int) {
    cells := s.current()
    for i := 0; i < n; i++ {
        top := s.ScrollTop * s.Cols
        bot := (s.ScrollBottom+1)*s.Cols - 1
        // Shift rows up within scroll region
        for r := s.ScrollTop; r < s.ScrollBottom; r++ {
            dst := r * s.Cols
            src := (r + 1) * s.Cols
            copy(cells[dst:dst+s.Cols], cells[src:src+s.Cols])
        }
        // Clear bottom line
        b := s.ScrollBottom * s.Cols
        for j := b; j < b+s.Cols; j++ {
            cells[j] = EmptyCell()
        }
        _ = bot
        _ = top
    }
}

// scrollDown scrolls the scroll region down by n lines.
func (s *Screen) scrollDown(n int) {
    cells := s.current()
    for i := 0; i < n; i++ {
        for r := s.ScrollBottom; r > s.ScrollTop; r-- {
            dst := r * s.Cols
            src := (r - 1) * s.Cols
            copy(cells[dst:dst+s.Cols], cells[src:src+s.Cols])
        }
        t := s.ScrollTop * s.Cols
        for j := t; j < t+s.Cols; j++ {
            cells[j] = EmptyCell()
        }
    }
}

// SwitchAltBuffer toggles between main and alternate screen buffer.
func (s *Screen) SwitchAltBuffer(use bool) {
    s.UseAltBuffer = use
}

// RenderText renders the screen as plain text.
func (s *Screen) RenderText() string {
    var buf bytes.Buffer
    cells := s.current()
    for r := 0; r < s.Rows; r++ {
        start := r * s.Cols
        for c := 0; c < s.Cols; c++ {
            ch := cells[start+c].Rune
            if ch == 0 {
                ch = ' '
            }
            buf.WriteRune(ch)
        }
        if r < s.Rows-1 {
            buf.WriteByte('\n')
        }
    }
    return buf.String()
}

// CellsView returns a read-only slice of the active buffer for inspection.
// Index with [row*Cols + col].
func (s *Screen) CellsView() []Cell {
    return s.current()
}

func clamp(v, min, max int) int {
    if v < min {
        return min
    }
    if v > max {
        return max
    }
    return v
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}
