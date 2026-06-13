package cue

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLaunchEcho(t *testing.T) {
	app, err := Launch("echo", WithArgs("hello", "world"))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	// Wait for process to finish and output to be captured
	time.Sleep(200 * time.Millisecond)

	page := app.Page()
	text, err := page.Text()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(text, "hello world") {
		t.Fatalf("expected 'hello world' in output, got:\n%s", text)
	}
}

func TestLaunchCat(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(fpath, []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	app, err := Launch("cat", WithArgs(fpath))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	time.Sleep(200 * time.Millisecond)

	page := app.Page()
	lines, err := page.Lines()
	if err != nil {
		t.Fatal(err)
	}

	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "hello") {
		t.Fatalf("expected 'hello' on line 0, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "world") {
		t.Fatalf("expected 'world' on line 1, got %q", lines[1])
	}
}

func TestSaveArtifact(t *testing.T) {
	app, err := Launch("echo", WithArgs("artifact"))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	time.Sleep(200 * time.Millisecond)

	page := app.Page()
	dir := t.TempDir()
	if err := page.SaveArtifact(dir, "step1"); err != nil {
		t.Fatal(err)
	}

	render, err := os.ReadFile(filepath.Join(dir, "step1.render.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(render), "artifact") {
		t.Fatalf("expected 'artifact' in render, got %q", string(render))
	}

	state, err := os.ReadFile(filepath.Join(dir, "step1.state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(state) == 0 {
		t.Fatal("expected non-empty state.json")
	}
}

func TestMouseSGR(t *testing.T) {
	// Smoke test: ensure MouseSGR generates valid sequences
	seq := MouseSGR(0, 10, 5, false)
	if !strings.Contains(string(seq), "\x1b[") {
		t.Fatalf("expected CSI prefix, got %q", string(seq))
	}
	if !strings.Contains(string(seq), "M") {
		t.Fatalf("expected press suffix M, got %q", string(seq))
	}

	seqUp := MouseSGR(64, 0, 0, false)
	if !strings.Contains(string(seqUp), "64") {
		t.Fatalf("expected button 64 (scroll up), got %q", string(seqUp))
	}
}
