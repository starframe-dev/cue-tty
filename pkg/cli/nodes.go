package cli

import (
	"fmt"
	"strings"

	"github.com/starframe-dev/cue-tty/pkg/cue"
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

// CellNode represents a single cell with its content and style.
type CellNode struct {
	Row, Col int
	Rune     rune
	FG       string
	BG       string
	Bold     bool
}

// BuildNodeTree builds a node tree from the current page state.
func BuildNodeTree(page *cue.Page) (*NodeTree, error) {
	lines, err := page.Lines()
	if err != nil {
		return nil, err
	}

	row, col, err := page.Cursor()
	if err != nil {
		return nil, err
	}

	nt := &NodeTree{
		Rows: len(lines),
		Cols: 80,
		Cursor: struct {
			Row, Col int
		}{Row: row, Col: col},
	}

	if len(lines) > 0 {
		nt.Cols = len(lines[0])
	}

	for i, line := range lines {
		nt.Lines = append(nt.Lines, LineNode{
			Index: i,
			Text:  line,
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
		display := ln.Text
		if len(display) > 60 {
			display = display[:60] + "..."
		}
		// Escape non-printable
		display = escapeNonPrintable(display)
		b.WriteString(fmt.Sprintf("    %s [%2d] %q\n", prefix, ln.Index, display))
	}

	return b.String()
}

func escapeNonPrintable(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 32 && r <= 126 {
			b.WriteRune(r)
		} else if r == '\n' {
			b.WriteString("\\n")
		} else if r == '\t' {
			b.WriteString("\\t")
		} else if r == '\r' {
			b.WriteString("\\r")
		} else {
			b.WriteString(fmt.Sprintf("\\x%02x", r))
		}
	}
	return b.String()
}
