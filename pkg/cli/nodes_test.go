package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/starframe-dev/cue-tty/pkg/cue"
)

func TestBuildNodeTree(t *testing.T) {
	app, err := cue.Launch("echo", cue.WithArgs("hello"), cue.WithSize(40, 10))
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	defer app.Close()

	page := app.Page()
	time.Sleep(100 * time.Millisecond)

	nt, err := BuildNodeTree(page)
	if err != nil {
		t.Fatalf("BuildNodeTree: %v", err)
	}

	if nt.Rows != 10 {
		t.Errorf("expected 10 rows, got %d", nt.Rows)
	}
	if nt.Cols != 40 {
		t.Errorf("expected 40 cols, got %d", nt.Cols)
	}
	if len(nt.Lines) != 10 {
		t.Errorf("expected 10 lines, got %d", len(nt.Lines))
	}
	if !strings.Contains(nt.Lines[0].Text, "hello") {
		t.Errorf("expected first line to contain 'hello', got %q", nt.Lines[0].Text)
	}
}

func TestNodeTreeRender(t *testing.T) {
	nt := &NodeTree{
		Cols: 40,
		Rows: 10,
		Cursor: struct {
			Row, Col int
		}{Row: 5, Col: 10},
		Lines: []LineNode{
			{Index: 0, Text: "hello"},
			{Index: 1, Text: "world"},
		},
	}

	rendered := nt.Render()
	if !strings.Contains(rendered, "Screen 40x10") {
		t.Errorf("expected screen size in render, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Cursor: (5, 10)") {
		t.Errorf("expected cursor in render, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "[ 0] \"hello\"") {
		t.Errorf("expected line 0 in render, got:\n%s", rendered)
	}
}

func TestEscapeNonPrintable(t *testing.T) {
	got := escapeNonPrintable("a\nb\tc\rd")
	want := "a\\nb\\tc\\rd"
	if got != want {
		t.Errorf("escapeNonPrintable = %q, want %q", got, want)
	}
}
