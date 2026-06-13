package cue

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func buildBubbleApp(t *testing.T) string {
	t.Helper()
	_, here, _, _ := runtime.Caller(0)
	srcDir := filepath.Join(filepath.Dir(here), "..", "..", "testdata", "bubble")
	binPath := filepath.Join(t.TempDir(), "bubble")

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = srcDir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build bubble app: %v\n%s", err, out)
	}
	return binPath
}

func TestBubbleteaIntegration(t *testing.T) {
	bin := buildBubbleApp(t)

	app, err := Launch(bin, WithSize(80, 24))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	page := app.Page()

	// Give Bubbletea time to initialize and render
	page.WaitStable(100 * time.Millisecond)

	// Enable mouse tracking
	page.EnableMouse()
	page.WaitStable(500 * time.Millisecond)

	// Verify initial render
	if err := Expect(page).ToContain("Pick an item"); err != nil {
		t.Fatalf("initial render failed:\n%s", mustText(page))
	}
	if err := Expect(page).ToContain("> Apple"); err != nil {
		t.Fatal(err)
	}

	// Test keyboard navigation: Down
	page.Press("Down")
	page.WaitStable(100 * time.Millisecond)

	if err := Expect(page).ToContain("> Banana"); err != nil {
		t.Fatalf("expected > Banana after Down, got:\n%s", mustText(page))
	}

	// Test Enter to select
	page.Press("Enter")
	page.WaitStable(100 * time.Millisecond)

	if err := Expect(page).ToContain("Selected: Banana"); err != nil {
		t.Fatalf("expected 'Selected: Banana' after Enter, got:\n%s", mustText(page))
	}

	// Test mouse click on Cherry
	page.MouseClick(4, 4)
	page.WaitStable(100 * time.Millisecond)

	if err := Expect(page).ToContain("> Cherry"); err != nil {
		t.Fatalf("expected > Cherry after click, got:\n%s", mustText(page))
	}

	// Quit
	page.Press("q")
	page.WaitStable(100 * time.Millisecond)

	done := make(chan error, 1)
	go func() { done <- app.Wait() }()
	select {
	case <-done:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("app did not exit after q")
	}
}

func mustText(p *Page) string {
	text, _ := p.Text()
	return text
}
