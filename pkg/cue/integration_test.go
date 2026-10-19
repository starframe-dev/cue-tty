package cue

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
	app, err := Launch("yes")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(150 * time.Millisecond)
	if got := len(app.RawLog()); got > maxRawLogBytes {
		t.Fatalf("raw log exceeded limit: %d > %d", got, maxRawLogBytes)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
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
