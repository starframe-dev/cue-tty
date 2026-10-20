package cli

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/rivo/uniseg"
	"github.com/starframe-dev/cue-tty/pkg/cue"
	"github.com/starframe-dev/portalis"
)

// NodeTree represents the visual structure of the screen.
type NodeTree struct {
	Cols   int
	Rows   int
	Cursor struct {
		Row, Col int
	}
	Lines []LineNode
	Cells []CellNode
}

// LineNode represents a single line on screen.
type LineNode struct {
	Index int
	Text  string
}

// CellNode represents a terminal cell and its style.
type CellNode struct {
	Row, Col      int
	Rune          rune
	Combining     string
	Continuation  bool
	FG            string
	BG            string
	Bold          bool
	Dim           bool
	Italic        bool
	Underline     bool
	Reverse       bool
	Strikethrough bool
}

// BuildNodeTree builds one internally consistent view of the screen.
func BuildNodeTree(page *cue.Page) (*NodeTree, error) {
	snapshot := page.TerminalSnapshot()
	lines := strings.Split(snapshot.Text, "\n")
	nt := &NodeTree{
		Rows: snapshot.Rows,
		Cols: snapshot.Cols,
		Cursor: struct {
			Row, Col int
		}{Row: snapshot.CursorRow, Col: snapshot.CursorCol},
	}
	for i, line := range lines {
		nt.Lines = append(nt.Lines, LineNode{Index: i, Text: line})
	}
	for i, cell := range snapshot.Cells {
		row, col := i/snapshot.Cols, i%snapshot.Cols
		nt.Cells = append(nt.Cells, CellNode{
			Row: row, Col: col, Rune: cell.Rune, Combining: cell.Combining,
			Continuation: cell.Continuation, FG: string(cell.FG), BG: string(cell.BG),
			Bold:          cell.Style&portalis.StyleBold != 0,
			Dim:           cell.Style&portalis.StyleDim != 0,
			Italic:        cell.Style&portalis.StyleItalic != 0,
			Underline:     cell.Style&portalis.StyleUnderline != 0,
			Reverse:       cell.Style&portalis.StyleReverse != 0,
			Strikethrough: cell.Style&portalis.StyleStrikethrough != 0,
		})
	}
	return nt, nil
}

// Render renders the node tree as a string.
func (nt *NodeTree) Render() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Screen %dx%d\n", nt.Cols, nt.Rows))
	b.WriteString(fmt.Sprintf("├── Cursor: (%d, %d)\n", nt.Cursor.Row, nt.Cursor.Col))
	b.WriteString(fmt.Sprintf("├── Lines (%d):\n", len(nt.Lines)))

	for i, ln := range nt.Lines {
		prefix := "│   ├──"
		if i == len(nt.Lines)-1 {
			prefix = "│   └──"
		}
		display := escapeNonPrintable(truncateDisplayWidth(ln.Text, 60))
		b.WriteString(fmt.Sprintf("    %s [%2d] %q\n", prefix, ln.Index, display))
	}

	return b.String()
}

// RenderCells renders only occupied, styled, or wide-character continuation
// cells. Blank default cells are omitted to keep large screen dumps readable.
func (nt *NodeTree) RenderCells() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Screen %dx%d\n", nt.Cols, nt.Rows))
	b.WriteString(fmt.Sprintf("Cursor: (%d, %d)\n", nt.Cursor.Row, nt.Cursor.Col))
	b.WriteString("Cells (row, col; zero-based):\n")

	count := 0
	for _, cell := range nt.Cells {
		if cell.isDefaultBlank() {
			continue
		}
		char := ""
		if cell.Rune != 0 {
			char = string(cell.Rune)
		}
		b.WriteString(fmt.Sprintf(
			"  [%d,%d] char=%q combining=%q fg=%q bg=%q attrs=[%s] continuation=%t\n",
			cell.Row, cell.Col, char, cell.Combining, cell.FG, cell.BG,
			strings.Join(cell.styleAttributes(), ","), cell.Continuation,
		))
		count++
	}
	if count == 0 {
		b.WriteString("  (no occupied or styled cells)\n")
	}
	return b.String()
}

func (cell CellNode) isDefaultBlank() bool {
	hasStyle := cell.Bold || cell.Dim || cell.Italic || cell.Underline || cell.Reverse || cell.Strikethrough
	if cell.Continuation || cell.Combining != "" || cell.FG != "" || cell.BG != "" || hasStyle {
		return false
	}
	return cell.Rune == 0 || cell.Rune == ' '
}

func (cell CellNode) styleAttributes() []string {
	var attrs []string
	if cell.Bold {
		attrs = append(attrs, "bold")
	}
	if cell.Dim {
		attrs = append(attrs, "dim")
	}
	if cell.Italic {
		attrs = append(attrs, "italic")
	}
	if cell.Underline {
		attrs = append(attrs, "underline")
	}
	if cell.Reverse {
		attrs = append(attrs, "reverse")
	}
	if cell.Strikethrough {
		attrs = append(attrs, "strikethrough")
	}
	return attrs
}

func truncateDisplayWidth(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if uniseg.StringWidth(text) <= maxWidth {
		return text
	}
	targetWidth := maxWidth - 1
	var out strings.Builder
	width := 0
	graphemes := uniseg.NewGraphemes(text)
	for graphemes.Next() {
		cluster := graphemes.Str()
		clusterWidth := uniseg.StringWidth(cluster)
		if width+clusterWidth > targetWidth {
			break
		}
		out.WriteString(cluster)
		width += clusterWidth
	}
	return out.String() + "…"
}

func escapeNonPrintable(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\n' {
			b.WriteString("\\n")
		} else if r == '\t' {
			b.WriteString("\\t")
		} else if r == '\r' {
			b.WriteString("\\r")
		} else if unicode.IsPrint(r) {
			b.WriteRune(r)
		} else {
			b.WriteString(fmt.Sprintf("\\u%04x", r))
		}
	}
	return b.String()
}
