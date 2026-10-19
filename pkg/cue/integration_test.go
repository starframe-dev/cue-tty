package cue

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestLaunchEcho(t *testing.T) {
	app, err := Launch("echo", WithArgs("hello", "world"))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	page := app.Page()
	if err := page.WaitFor("hello world", time.Second); err != nil {
		t.Fatal(err)
	}
	text, err := page.Text()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(text, "hello world") {
		t.Fatalf("expected 'hello world' in output, got:\n%s", text)
	}
}

func TestTypeContextCancelsBlockedPTYWrite(t *testing.T) {
	app, err := Launch("sh", WithArgs("-c", "exec sleep 30"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = app.Page().TypeContext(ctx, strings.Repeat("x", 4<<20))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("TypeContext error = %v, want deadline exceeded", err)
	}
	if time.Since(started) > 3*time.Second {
		t.Fatalf("blocked PTY input cancellation took too long: %s", time.Since(started))
	}
	select {
	case <-app.done:
	case <-time.After(2 * time.Second):
		t.Fatal("PTY reader remained blocked after canceled input")
	}
	if app.backend.ProcessState() == nil {
		t.Fatal("canceled input did not reap the application process")
	}
}

func TestResizeValidationPreservesPTYAndScreen(t *testing.T) {
	app, err := Launch("sleep", WithArgs("10"), WithSize(80, 24))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	invalid := [][2]int{{0, 24}, {80, 0}, {-1, 24}, {80, -1}, {65536, 24}, {4097, 1}, {2048, 1024}}
	for _, size := range invalid {
		if err := app.Resize(size[0], size[1]); err == nil {
			t.Errorf("Resize(%d, %d) unexpectedly succeeded", size[0], size[1])
		}
	}
	cols, rows, err := app.backend.Size()
	if err != nil {
		t.Fatal(err)
	}
	snapshot := app.TerminalSnapshot()
	if cols != 80 || rows != 24 || snapshot.Cols != 80 || snapshot.Rows != 24 {
		t.Fatalf("invalid resize changed terminal: PTY=%dx%d screen=%dx%d", cols, rows, snapshot.Cols, snapshot.Rows)
	}
}

func TestConcurrentResizeKeepsPTYAndScreenConsistent(t *testing.T) {
	app, err := Launch("sleep", WithArgs("10"), WithSize(80, 24))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	sizes := [][2]int{{80, 24}, {100, 30}, {120, 40}, {1024, 128}}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for n := 0; n < 20; n++ {
				size := sizes[(offset+n)%len(sizes)]
				if err := app.Resize(size[0], size[1]); err != nil {
					t.Errorf("Resize(%d, %d): %v", size[0], size[1], err)
				}
			}
		}(i)
	}
	wg.Wait()
	cols, rows, err := app.backend.Size()
	if err != nil {
		t.Fatal(err)
	}
	snapshot := app.TerminalSnapshot()
	if cols != snapshot.Cols || rows != snapshot.Rows {
		t.Fatalf("PTY and screen size differ: PTY=%dx%d screen=%dx%d", cols, rows, snapshot.Cols, snapshot.Rows)
	}
}

func TestWithTimeoutKillsProcess(t *testing.T) {
	app, err := Launch("sleep", WithArgs("10"), WithTimeout(100*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	select {
	case <-app.done:
	case <-time.After(3 * time.Second):
		t.Fatal("application did not stop after timeout")
	}
	_ = app.Wait()
	if app.backend.ProcessState() == nil {
		t.Fatal("timed out process was not reaped")
	}
}

func TestCloseKillsPTYProcessGroup(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	script := "sleep 60 & echo $! > " + pidFile + "; wait"
	app, err := Launch("sh", WithArgs("-c", script), WithDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	var childPID int
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			childPID, _ = strconv.Atoi(strings.TrimSpace(string(data)))
			if childPID > 0 {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if childPID <= 0 {
		t.Fatal("child process did not start")
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(childPID, 0); err == syscall.ESRCH {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("child process %d remained alive after Close", childPID)
}

func TestCloseIsIdempotent(t *testing.T) {
	app, err := Launch("sleep", WithArgs("10"))
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-app.done:
	case <-time.After(time.Second):
		t.Fatal("PTY reader remained running after Close")
	}
}

func TestRawLogIsBounded(t *testing.T) {
	app := &App{}
	app.appendRawLog(bytes.Repeat([]byte{'a'}, maxRawLogBytes-1))
	app.appendRawLog([]byte("bc"))
	if got := len(app.rawLog); got != maxRawLogBytes {
		t.Fatalf("raw log length = %d, want %d", got, maxRawLogBytes)
	}
	if app.rawLog[0] != 'a' || string(app.rawLog[len(app.rawLog)-2:]) != "bc" {
		t.Fatalf("raw log did not retain the newest output tail")
	}

	oversized := bytes.Repeat([]byte{'z'}, maxRawLogBytes+17)
	app.appendRawLog(oversized)
	if got := len(app.rawLog); got != maxRawLogBytes {
		t.Fatalf("oversized append produced %d bytes, want %d", got, maxRawLogBytes)
	}
	if !bytes.Equal(app.rawLog, oversized[len(oversized)-maxRawLogBytes:]) {
		t.Fatal("oversized append did not retain the final maxRawLogBytes bytes")
	}
}

func TestRelativeExecutableRunsFromWorkDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "relative-app")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf 'relative-ok\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	app, err := Launch("./relative-app", WithDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if err := app.Page().WaitFor("relative-ok", time.Second); err != nil {
		t.Fatal(err)
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

	page := app.Page()
	if err := page.WaitFor("world", time.Second); err != nil {
		t.Fatal(err)
	}
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

	page := app.Page()
	if err := page.WaitFor("artifact", time.Second); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := page.SaveArtifact(dir, "step1"); err != nil {
		t.Fatal(err)
	}
	artifactInfo, err := os.Stat(filepath.Join(dir, "step1.render.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if artifactInfo.Mode().Perm()&0o077 != 0 {
		t.Fatalf("artifact permissions are too broad: %o", artifactInfo.Mode().Perm())
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

func TestSaveArtifactDoesNotFollowSymlink(t *testing.T) {
	app, err := Launch("sleep", WithArgs("5"))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "outside.txt")
	if err := os.WriteFile(target, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "snapshot.render.txt")); err != nil {
		t.Fatal(err)
	}
	if err := app.Page().SaveArtifact(dir, "snapshot"); err == nil {
		t.Fatal("expected symlink destination to be rejected")
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "keep" {
		t.Fatalf("symlink target was changed: data=%q err=%v", data, err)
	}
}

func TestSaveArtifactRejectsPathTraversal(t *testing.T) {
	app, err := Launch("echo", WithArgs("artifact"))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if err := app.Wait(); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	for _, name := range []string{"../escape", "nested/name", ".", ""} {
		if err := app.Page().SaveArtifact(dir, name); err == nil {
			t.Errorf("SaveArtifact(%q) unexpectedly succeeded", name)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "escape.render.txt")); !os.IsNotExist(err) {
		t.Fatalf("traversal artifact unexpectedly exists: %v", err)
	}
}

func TestLineRejectsInvalidIndex(t *testing.T) {
	app, err := Launch("sleep", WithArgs("5"))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if _, err := app.Page().Line(-1); err == nil {
		t.Fatal("negative line index should return an error")
	}
	if _, err := app.Page().Line(24); err == nil {
		t.Fatal("line index beyond the screen should return an error")
	}
}

func TestInvalidKeyAndMouseCoordinatesReturnErrors(t *testing.T) {
	app, err := Launch("sleep", WithArgs("5"))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	page := app.Page()
	if err := page.Press("NotAKey"); err == nil {
		t.Fatal("unknown key should return an error")
	}
	if err := page.MouseClick(-1, 0); err == nil {
		t.Fatal("negative mouse coordinate should return an error")
	}
	if err := page.MouseMove(80, 0); err == nil {
		t.Fatal("out-of-range mouse coordinate should return an error")
	}
}

func TestMouseButtonStateAndMouseUpValidation(t *testing.T) {
	app, err := Launch("sleep", WithArgs("10"))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	page := app.Page()

	for button := 0; button <= 2; button++ {
		if err := page.MouseDown(button, 1, 1); err != nil {
			t.Fatal(err)
		}
		if page.dragBtn != button {
			t.Fatalf("drag button after down = %d, want %d", page.dragBtn, button)
		}
		if err := page.MouseMove(2, 2); err != nil {
			t.Fatal(err)
		}
		if err := page.MouseUp(button, 2, 2); err != nil {
			t.Fatal(err)
		}
		if page.dragBtn != -1 {
			t.Fatalf("drag button after release = %d, want none", page.dragBtn)
		}
	}
	if err := page.MouseDown(1, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err := page.MouseUp(2, 1, 1); err != nil {
		t.Fatal(err)
	}
	if page.dragBtn != 1 {
		t.Fatalf("releasing a different button cleared active drag: %d", page.dragBtn)
	}
	if err := page.MouseUp(3, 1, 1); err == nil {
		t.Fatal("MouseUp accepted an invalid button")
	}
	if err := page.MouseUp(1, 1, 1); err != nil {
		t.Fatal(err)
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
