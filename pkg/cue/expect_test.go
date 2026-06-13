package cue

import (
	"testing"

	"github.com/starframe-dev/cue-tty/pkg/ansi"
)

func newTestPage(text string) *Page {
	app := &App{
		screen: ansi.NewScreen(80, 24),
	}
	p := ansi.NewParser(app.screen)
	p.FeedString(text)
	return app.Page()
}

func TestExpectToContain(t *testing.T) {
	page := newTestPage("Hello World")
	if err := Expect(page).ToContain("World"); err != nil {
		t.Fatal(err)
	}
	if err := Expect(page).ToContain("Missing"); err == nil {
		t.Fatal("expected error for missing text")
	}
}

func TestExpectToMatch(t *testing.T) {
	page := newTestPage("Error: 404 Not Found")
	if err := Expect(page).ToMatch(`Error: \d+`); err != nil {
		t.Fatal(err)
	}
	if err := Expect(page).ToMatch(`^Error:`); err != nil {
		t.Fatal(err)
	}
}

func TestExpectToHaveLine(t *testing.T) {
	page := newTestPage("Line1\r\nLine2")
	if err := Expect(page).ToHaveLine(0, "Line1"); err != nil {
		t.Fatal(err)
	}
	if err := Expect(page).ToHaveLine(1, "Line2"); err != nil {
		t.Fatal(err)
	}
	if err := Expect(page).ToHaveLine(0, "Wrong"); err == nil {
		t.Fatal("expected error")
	}
}

func TestExpectToHaveCursorAt(t *testing.T) {
	app := &App{screen: ansi.NewScreen(80, 24)}
	app.screen.MoveCursor(5, 10)
	page := app.Page()
	if err := Expect(page).ToHaveCursorAt(5, 10); err != nil {
		t.Fatal(err)
	}
	if err := Expect(page).ToHaveCursorAt(0, 0); err == nil {
		t.Fatal("expected error")
	}
}
