package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// buildDemoApp builds the warp-demo binary in a temp directory.
func buildDemoApp(t *testing.T) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "warp-demo")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = "/Users/a/Space/Projects/Starframe/cue-tty/examples/warp-demo"
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build warp-demo: %v\n%s", err, out)
	}
	return binPath
}

// cuetty runs a single cuetty-cli command and returns the combined output.
func cuetty(t *testing.T, session string, args ...string) string {
	t.Helper()

	cmdArgs := append([]string{"-s", session}, args...)
	cmd := exec.Command("cuetty-cli", cmdArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cuetty-cli %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// cuettyInDir runs cuetty-cli from a specific working directory.
func cuettyInDir(t *testing.T, dir, session string, args ...string) string {
	t.Helper()

	cmdArgs := append([]string{"-s", session}, args...)
	cmd := exec.Command("cuetty-cli", cmdArgs...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cuetty-cli %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// TestLaunchAndText verifies that warp-demo launches and renders the UI.
func TestLaunchAndText(t *testing.T) {
	binPath := buildDemoApp(t)
	session := "launch-text-" + t.Name()

	cuetty(t, session, "launch", binPath)
	cuetty(t, session, "wait", "500")
	out := cuetty(t, session, "expect", "contain", "Warp Demo")

	if !strings.Contains(out, "✅ Found") {
		t.Errorf("expected to find 'Warp Demo', got:\n%s", out)
	}
	out = cuetty(t, session, "expect", "contain", "Item 1")
	if !strings.Contains(out, "✅ Found") {
		t.Errorf("expected to find 'Item 1', got:\n%s", out)
	}
	out = cuetty(t, session, "expect", "contain", "[Add]")
	if !strings.Contains(out, "✅ Found") {
		t.Errorf("expected to find '[Add]', got:\n%s", out)
	}

	cuetty(t, session, "exit")
}

// TestElementTree verifies that node shows the HTTP element tree.
func TestElementTree(t *testing.T) {
	binPath := buildDemoApp(t)
	session := "element-tree-" + t.Name()

	cuetty(t, session, "launch", binPath)
	cuetty(t, session, "wait", "500")
	out := cuetty(t, session, "node")

	expected := []string{
		"<header> \"Warp Demo\"",
		"<listitem> \"> Item 1\"",
		"<button> \"Add\"",
		"<button> \"Remove\"",
		"<button> \"Select\"",
	}
	for _, exp := range expected {
		if !strings.Contains(out, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, out)
		}
	}

	cuetty(t, session, "exit")
}

// TestClickButtonAdd verifies that clicking the Add button adds an item.
func TestClickButtonAdd(t *testing.T) {
	binPath := buildDemoApp(t)
	session := "click-add-" + t.Name()

	cuetty(t, session, "launch", binPath)
	cuetty(t, session, "wait", "500")
	cuetty(t, session, "click", "button:Add")
	cuetty(t, session, "wait", "200")

	out := cuetty(t, session, "expect", "contain", "Item 4")
	if !strings.Contains(out, "✅ Found") {
		t.Errorf("expected to find 'Item 4', got:\n%s", out)
	}
	out = cuetty(t, session, "expect", "contain", "Added 4 items total")
	if !strings.Contains(out, "✅ Found") {
		t.Errorf("expected to find 'Added 4 items total', got:\n%s", out)
	}

	cuetty(t, session, "exit")
}

// TestClickListItem verifies that clicking a list item selects it.
func TestClickListItem(t *testing.T) {
	binPath := buildDemoApp(t)
	session := "click-item-" + t.Name()

	cuetty(t, session, "launch", binPath)
	cuetty(t, session, "wait", "500")
	cuetty(t, session, "click", "listitem:\"  Item 2\"")
	cuetty(t, session, "wait", "200")

	out := cuetty(t, session, "expect", "contain", "Selected: Item 2")
	if !strings.Contains(out, "✅ Found") {
		t.Errorf("expected to find 'Selected: Item 2', got:\n%s", out)
	}

	cuetty(t, session, "exit")
}

// TestKeyboardNavigation verifies Up/Down/Enter keyboard navigation.
func TestKeyboardNavigation(t *testing.T) {
	binPath := buildDemoApp(t)
	session := "keyboard-" + t.Name()

	cuetty(t, session, "launch", binPath)
	cuetty(t, session, "wait", "500")
	cuetty(t, session, "press", "Down")
	cuetty(t, session, "press", "Down")
	cuetty(t, session, "press", "Enter")
	cuetty(t, session, "wait", "200")

	out := cuetty(t, session, "expect", "contain", "Selected: Item 3")
	if !strings.Contains(out, "✅ Found") {
		t.Errorf("expected to find 'Selected: Item 3', got:\n%s", out)
	}

	cuetty(t, session, "exit")
}

// TestSessionIsolation verifies that two sessions are independent.
func TestSessionIsolation(t *testing.T) {
	binPath := buildDemoApp(t)
	sessionA := "session-a-" + t.Name()
	sessionB := "session-b-" + t.Name()

	cuetty(t, sessionA, "launch", binPath)
	cuetty(t, sessionA, "wait", "500")
	cuetty(t, sessionA, "click", "button:Add")
	cuetty(t, sessionA, "wait", "200")
	out := cuetty(t, sessionA, "expect", "contain", "Added 4 items total")
	if !strings.Contains(out, "✅ Found") {
		t.Errorf("session-a expected to find 'Added 4 items total', got:\n%s", out)
	}

	cuetty(t, sessionB, "launch", binPath)
	cuetty(t, sessionB, "wait", "500")
	out = cuetty(t, sessionB, "expect", "not", "Item 4")
	if !strings.Contains(out, "✅ Not found") {
		t.Errorf("session-b expected not to find 'Item 4', got:\n%s", out)
	}

	cuetty(t, sessionA, "exit")
	cuetty(t, sessionB, "exit")
}

// TestSaveArtifact verifies that save creates render.txt and state.json.
func TestSaveArtifact(t *testing.T) {
	binPath := buildDemoApp(t)
	artifactDir := filepath.Join(t.TempDir(), "artifacts")
	_ = os.MkdirAll(artifactDir, 0755)
	session := "save-test-" + t.Name()

	cuettyInDir(t, artifactDir, session, "launch", binPath)
	cuettyInDir(t, artifactDir, session, "wait", "500")
	cuettyInDir(t, artifactDir, session, "text")
	out := cuettyInDir(t, artifactDir, session, "save", "step1")

	if !strings.Contains(out, "Saved to") {
		t.Errorf("expected save output to contain 'Saved to', got:\n%s", out)
	}

	renderPath := filepath.Join(artifactDir, "cuetty-artifacts", session, "step1", "render.txt")
	statePath := filepath.Join(artifactDir, "cuetty-artifacts", session, "step1", "state.json")

	if _, err := os.Stat(renderPath); err != nil {
		t.Errorf("expected render.txt to exist: %v", err)
	}
	if _, err := os.Stat(statePath); err != nil {
		t.Errorf("expected state.json to exist: %v", err)
	}

	render, err := os.ReadFile(renderPath)
	if err != nil {
		t.Fatalf("read render.txt: %v", err)
	}
	if !strings.Contains(string(render), "Warp Demo") {
		t.Errorf("expected render.txt to contain 'Warp Demo', got:\n%s", render)
	}

	cuettyInDir(t, artifactDir, session, "exit")
}

// TestMultipleClicks verifies multiple add/remove operations.
func TestMultipleClicks(t *testing.T) {
	binPath := buildDemoApp(t)
	session := "multi-click-" + t.Name()

	cuetty(t, session, "launch", binPath)
	cuetty(t, session, "wait", "500")
	cuetty(t, session, "click", "button:Add")
	cuetty(t, session, "wait", "200")
	cuetty(t, session, "click", "button:Add")
	cuetty(t, session, "wait", "200")

	out := cuetty(t, session, "expect", "contain", "Item 5")
	if !strings.Contains(out, "✅ Found") {
		t.Errorf("expected to find 'Item 5', got:\n%s", out)
	}

	cuetty(t, session, "click", "listitem:\"  Item 5\"")
	cuetty(t, session, "wait", "200")
	cuetty(t, session, "click", "button:Remove")
	cuetty(t, session, "wait", "200")

	out = cuetty(t, session, "expect", "contain", "Removed: Item 5")
	if !strings.Contains(out, "✅ Found") {
		t.Errorf("expected to find 'Removed: Item 5', got:\n%s", out)
	}

	cuetty(t, session, "exit")
}

// TestCLISessionsList verifies that cuetty-cli list and show work.
func TestCLISessionsList(t *testing.T) {
	binPath := buildDemoApp(t)
	session := "list-show-" + t.Name()

	cuetty(t, session, "launch", binPath)
	cuetty(t, session, "wait", "500")
	out := cuetty(t, "", "list")
	if !strings.Contains(out, session) {
		t.Errorf("expected list to contain session %q, got:\n%s", session, out)
	}

	out = cuetty(t, "", "show")
	if !strings.Contains(out, "Active sessions") {
		t.Errorf("expected show output to contain 'Active sessions', got:\n%s", out)
	}

	cuetty(t, session, "exit")
}

// TestStopDaemon verifies that the daemon can be stopped.
func TestStopDaemon(t *testing.T) {
	out, err := exec.Command("cuetty-cli", "-s", "default", "stop").CombinedOutput()
	if err != nil {
		t.Fatalf("stop daemon failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "Daemon stopped") {
		t.Errorf("expected 'Daemon stopped', got:\n%s", out)
	}
}
