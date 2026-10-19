package cli

import (
	"os"
	"path/filepath"
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
	if err := page.WaitFor("hello", time.Second); err != nil {
		t.Fatal(err)
	}

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

func TestBuildNodeTreePreservesUnicodeDimensionsCellsAndStyles(t *testing.T) {
	dir := t.TempDir()
	program := filepath.Join(dir, "unicode-app")
	script := "#!/bin/sh\nprintf '\\033[1;31m界é\\033[0m'\n"
	if err := os.WriteFile(program, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	app, err := cue.Launch(program, cue.WithSize(8, 2))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if err := app.Page().WaitFor("界é", time.Second); err != nil {
		t.Fatal(err)
	}

	tree, err := BuildNodeTree(app.Page())
	if err != nil {
		t.Fatal(err)
	}
	if tree.Cols != 8 || tree.Rows != 2 {
		t.Fatalf("NodeTree dimensions = %dx%d, want 8x2", tree.Cols, tree.Rows)
	}
	if len(tree.Cells) != 16 {
		t.Fatalf("NodeTree cells = %d, want 16", len(tree.Cells))
	}
	if tree.Cells[0].Rune != '界' || !tree.Cells[1].Continuation {
		t.Fatalf("wide rune cells were not preserved: %+v %+v", tree.Cells[0], tree.Cells[1])
	}
	if tree.Cells[2].Rune != 'e' || !strings.ContainsRune(tree.Cells[2].Combining, '\u0301') {
		t.Fatalf("combining sequence was not preserved: %+v", tree.Cells[2])
	}
	if !tree.Cells[0].Bold || tree.Cells[0].FG == "" {
		t.Fatalf("cell style was not exported: %+v", tree.Cells[0])
	}
}

func TestTruncateDisplayWidthDoesNotSplitGraphemes(t *testing.T) {
	got := truncateDisplayWidth("界éx", 3)
	if got != "界…" {
		t.Fatalf("truncateDisplayWidth = %q, want %q", got, "界…")
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
