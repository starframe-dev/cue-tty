package cli

import (
	"net/http"
	"net/http/httptest"
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

func TestCmdNodeFallsBackForOrdinaryTUI(t *testing.T) {
	session := &Session{Name: "ordinary"}
	if err := session.Launch("sh", "-c", "printf 'ordinary screen\\n'; exec sleep 30"); err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Page.WaitFor("ordinary screen", 2*time.Second); err != nil {
		t.Fatal(err)
	}

	started := time.Now()
	output, err := cmdNode(session, nil)
	if err != nil {
		t.Fatalf("node for ordinary TUI: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 4*time.Second {
		t.Fatalf("node fallback took %s", elapsed)
	}
	if !strings.Contains(output, "Screen 80x24") || !strings.Contains(output, "ordinary screen") {
		t.Fatalf("node did not return screen tree: %s", output)
	}
	if session.hasConfirmedInspector() {
		t.Fatal("silent non-Warp listener was marked as an available inspector")
	}
}

func TestCmdNodeUsesConfirmedElementEndpointAndPreservesErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/elements" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"role":"button","name":"Add","bounds":{"x":1,"y":2,"w":3,"h":4}}]`))
	}))

	session := &Session{Name: "warp", HTTPAddr: server.URL, httpToken: "test-token"}
	app, err := cue.Launch("sleep", cue.WithArgs("30"))
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	session.App = app
	session.Page = app.Page()
	defer session.Close()
	defer server.Close()

	output, err := cmdNode(session, nil)
	if err != nil {
		t.Fatalf("node for working inspector: %v", err)
	}
	if !strings.Contains(output, "Element tree:") || !strings.Contains(output, `<button> "Add"`) {
		t.Fatalf("node did not return element tree: %s", output)
	}
	if !session.hasConfirmedInspector() {
		t.Fatal("successful HTTP response did not confirm inspector availability")
	}

	server.Close()
	output, err = cmdNode(session, nil)
	if err == nil || !strings.Contains(err.Error(), "fetch element tree") {
		t.Fatalf("node after confirmed inspector failed without diagnostic: output=%q err=%v", output, err)
	}
	if strings.Contains(output, "Screen ") {
		t.Fatalf("node hid a confirmed inspector failure with screen fallback: %s", output)
	}
}

func TestCmdNodeReportsHTTPElementEndpointError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "inspector failed", http.StatusInternalServerError)
	}))
	defer server.Close()

	app, err := cue.Launch("sleep", cue.WithArgs("30"))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	session := &Session{App: app, Page: app.Page(), HTTPAddr: server.URL, httpToken: "token"}
	output, err := cmdNode(session, nil)
	if err == nil || !strings.Contains(err.Error(), "HTTP status 500") {
		t.Fatalf("node error = %v, want HTTP 500 diagnostic", err)
	}
	if strings.Contains(output, "Screen ") {
		t.Fatalf("HTTP inspector error was hidden by screen fallback: %s", output)
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
