package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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
	script := "#!/bin/sh\nprintf '\\033[1;31;44m界é\\033[0m'\n"
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
	if !tree.Cells[0].Bold || tree.Cells[0].FG == "" || tree.Cells[0].BG == "" {
		t.Fatalf("cell colors/style were not exported: %+v", tree.Cells[0])
	}

	detailed := tree.RenderCells()
	for _, expected := range []string{
		`[0,0] char="界"`,
		`[0,1] char="" combining=""`,
		`[0,2] char="e" combining=`,
		`fg=`, `bg=`, `attrs=[bold]`, `continuation=true`,
	} {
		if !strings.Contains(detailed, expected) {
			t.Errorf("detailed NodeTree does not contain %q:\n%s", expected, detailed)
		}
	}
	if !strings.Contains(detailed, "combining="+strconv.Quote("\u0301")) {
		t.Fatalf("detailed NodeTree omitted the combining mark:\n%s", detailed)
	}
	if got := strings.Count(detailed, "char="); got != 3 {
		t.Fatalf("detailed NodeTree emitted %d cells, want 3 non-empty/wide cells:\n%s", got, detailed)
	}
}

func TestNodeTreeRenderCellsSnapshot(t *testing.T) {
	tree := &NodeTree{
		Cols: 5,
		Rows: 1,
		Cursor: struct {
			Row, Col int
		}{Row: 0, Col: 3},
		Cells: []CellNode{
			{Row: 0, Col: 0, Rune: '界', FG: "#ff0000", BG: "#0000ff", Bold: true},
			{Row: 0, Col: 1, Continuation: true},
			{Row: 0, Col: 2, Rune: 'e', Combining: "\u0301"},
			{Row: 0, Col: 3, Rune: ' ', BG: "#222222", Underline: true},
			{Row: 0, Col: 4, Rune: ' '},
		},
	}

	want := "Screen 5x1\nCursor: (0, 3)\nCells (row, col; zero-based):\n" +
		"  [0,0] char=\"界\" combining=\"\" fg=\"#ff0000\" bg=\"#0000ff\" attrs=[bold] continuation=false\n" +
		"  [0,1] char=\"\" combining=\"\" fg=\"\" bg=\"\" attrs=[] continuation=true\n" +
		"  [0,2] char=\"e\" combining=" + strconv.Quote("\u0301") + " fg=\"\" bg=\"\" attrs=[] continuation=false\n" +
		"  [0,3] char=\" \" combining=\"\" fg=\"\" bg=\"#222222\" attrs=[underline] continuation=false\n"
	if got := tree.RenderCells(); got != want {
		t.Fatalf("RenderCells() snapshot mismatch:\n--- got ---\n%s--- want ---\n%s", got, want)
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
	if elapsed := time.Since(started); elapsed > 400*time.Millisecond {
		t.Fatalf("node fallback took %s, want a single bounded probe", elapsed)
	}
	if !strings.Contains(output, "Screen 80x24") || !strings.Contains(output, "ordinary screen") {
		t.Fatalf("node did not return screen tree: %s", output)
	}
	if session.hasConfirmedInspector() {
		t.Fatal("silent non-Warp listener was marked as an available inspector")
	}

	detailed, err := cmdNode(session, []string{"--cells"})
	if err != nil {
		t.Fatalf("node --cells: %v", err)
	}
	if !strings.Contains(detailed, "Cells (row, col; zero-based):") || !strings.Contains(detailed, `char="o"`) {
		t.Fatalf("node --cells did not render detailed screen cells: %s", detailed)
	}
	if _, err := cmdNode(session, []string{"--unknown"}); err == nil {
		t.Fatal("node accepted an unknown option")
	}
}

func TestCmdNodeFindsWarpInspectorAfterItStarts(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer late-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`[{"role":"header","name":"late Warp"}]`))
	}))
	defer server.Close()

	app, err := cue.Launch("sleep", cue.WithArgs("30"))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	session := &Session{App: app, Page: app.Page(), HTTPAddr: "http://" + server.Listener.Addr().String(), httpToken: "late-token"}

	started := time.Now()
	output, err := cmdNode(session, nil)
	if err != nil {
		t.Fatalf("node before inspector starts: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 400*time.Millisecond {
		t.Fatalf("node waited %s for an unstarted inspector", elapsed)
	}
	if !strings.Contains(output, "Screen 80x24") || session.hasConfirmedInspector() {
		t.Fatalf("unstarted inspector should return an unconfirmed screen tree: output=%s confirmed=%v", output, session.hasConfirmedInspector())
	}

	server.Start()
	output, err = cmdNode(session, nil)
	if err != nil {
		t.Fatalf("node after inspector starts: %v", err)
	}
	if !strings.Contains(output, "Element tree:") || !strings.Contains(output, "late Warp") || !session.hasConfirmedInspector() {
		t.Fatalf("late inspector was not discovered: output=%s confirmed=%v", output, session.hasConfirmedInspector())
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
