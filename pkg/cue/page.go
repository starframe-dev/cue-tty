package cue

import (
	"fmt"
	"strings"
	"time"
)

// Page provides Playwright-like API for interacting with a TUI page.
type Page struct {
	app *App
}

// Press sends a key press.
// Supports: "Enter", "Ctrl+C", "F1", "Up", "a", etc.
func (p *Page) Press(key string) error {
	seq := ResolveKey(key)
	if seq == nil {
		return nil
	}
	return p.app.write(seq)
}

// Type sends text character by character.
func (p *Page) Type(text string) error {
	for _, r := range text {
		if err := p.app.write([]byte(string(r))); err != nil {
			return err
		}
		time.Sleep(5 * time.Millisecond)
	}
	return nil
}

// Text returns the visible screen as plain text (no ANSI).
func (p *Page) Text() (string, error) {
	s := p.app.screenState()
	return s.RenderText(), nil
}

// Lines returns each row as a string.
func (p *Page) Lines() ([]string, error) {
	text, err := p.Text()
	if err != nil {
		return nil, err
	}
	return strings.Split(text, "\n"), nil
}

// Line returns a specific row (0-based).
func (p *Page) Line(n int) (string, error) {
	lines, err := p.Lines()
	if err != nil {
		return "", err
	}
	if n < 0 || n >= len(lines) {
		return "", nil
	}
	return lines[n], nil
}

// Cursor returns the current cursor position (row, col).
func (p *Page) Cursor() (row, col int, err error) {
	s := p.app.screenState()
	cur := s.CurrentCursor()
	return cur.Row, cur.Col, nil
}

// MouseClick sends a mouse click at (x, y) using SGR protocol.
// Coordinates are 0-based cell positions.
func (p *Page) MouseClick(x, y int) error {
	down := MouseSGR(0, x, y, false)
	up := MouseSGR(0, x, y, true)
	if err := p.app.write(down); err != nil {
		return err
	}
	return p.app.write(up)
}

// MouseScrollUp sends a scroll-up event at (x, y).
func (p *Page) MouseScrollUp(x, y int) error {
	return p.app.write(MouseSGR(64, x, y, false))
}

// MouseScrollDown sends a scroll-down event at (x, y).
func (p *Page) MouseScrollDown(x, y int) error {
	return p.app.write(MouseSGR(65, x, y, false))
}

// MouseDown sends a mouse button-down event.
// button: 0=left, 1=middle, 2=right.
func (p *Page) MouseDown(button, x, y int) error {
	return p.app.write(MouseSGR(button, x, y, false))
}

// MouseUp sends a mouse button-up event.
func (p *Page) MouseUp(button, x, y int) error {
	return p.app.write(MouseSGR(button, x, y, true))
}

// EnableMouse enables mouse tracking in the terminal (SGR protocol).
func (p *Page) EnableMouse() error {
	// Enable basic mouse tracking + SGR extended coordinates
	return p.app.write([]byte("\x1b[?1000h\x1b[?1006h\x1b[?1015h"))
}

// DisableMouse disables mouse tracking.
func (p *Page) DisableMouse() error {
	return p.app.write([]byte("\x1b[?1000l\x1b[?1006l\x1b[?1015l"))
}

// WaitFor waits until the screen contains the given text.
func (p *Page) WaitFor(text string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		t, err := p.Text()
		if err != nil {
			return err
		}
		if strings.Contains(t, text) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %q", text)
}

// WaitStable sleeps briefly to let the TUI settle.
func (p *Page) WaitStable(d time.Duration) {
	if d == 0 {
		d = 100 * time.Millisecond
	}
	time.Sleep(d)
}
