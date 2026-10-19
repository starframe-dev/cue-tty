package cue

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Page provides Playwright-like API for interacting with a TUI page.
type Page struct {
	app     *App
	mouseMu sync.Mutex
	dragBtn int // button currently held for drag (-1 if none)
	dragX   int // last reported drag X
	dragY   int // last reported drag Y
}

const pageWriteTimeout = 30 * time.Second

func (p *Page) withDefaultWriteContext(write func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), pageWriteTimeout)
	defer cancel()
	return write(ctx)
}

// Press sends a key press with a bounded default write timeout.
// Supports: "Enter", "Ctrl+C", "F1", "Up", "a", etc.
func (p *Page) Press(key string) error {
	return p.withDefaultWriteContext(func(ctx context.Context) error {
		return p.PressContext(ctx, key)
	})
}

// PressContext sends a key press with caller-controlled cancellation.
func (p *Page) PressContext(ctx context.Context, key string) error {
	seq := ResolveKey(key)
	if seq == nil {
		return fmt.Errorf("unknown key %q", key)
	}
	return p.app.writeContext(ctx, seq)
}

// Type writes text with a bounded default timeout.
func (p *Page) Type(text string) error {
	return p.withDefaultWriteContext(func(ctx context.Context) error {
		return p.TypeContext(ctx, text)
	})
}

// TypeContext writes text in order and aborts the app if the context expires
// while the PTY is backpressured.
func (p *Page) TypeContext(ctx context.Context, text string) error {
	return p.app.writeContext(ctx, []byte(text))
}

// Snapshot captures rendered text and cursor position atomically.
func (p *Page) Snapshot() ScreenSnapshot { return p.app.Snapshot() }

// TerminalSnapshot captures screen dimensions, text, cursor, and cells atomically.
func (p *Page) TerminalSnapshot() TerminalSnapshot { return p.app.TerminalSnapshot() }

// Text returns the visible screen as plain text (no ANSI).
func (p *Page) Text() (string, error) {
	return p.app.RenderText(), nil
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
		return "", fmt.Errorf("line index out of range: %d", n)
	}
	return lines[n], nil
}

// Cursor returns the current cursor position (row, col).
func (p *Page) Cursor() (row, col int, err error) {
	row, col = p.app.CursorPosition()
	return row, col, nil
}

// MouseClick sends a mouse click at (x, y) using SGR protocol.
// Coordinates are 0-based cell positions.
func (p *Page) MouseClick(x, y int) error {
	return p.withDefaultWriteContext(func(ctx context.Context) error {
		return p.MouseClickContext(ctx, x, y)
	})
}

func (p *Page) MouseClickContext(ctx context.Context, x, y int) error {
	if err := p.app.validateCoordinates(x, y); err != nil {
		return err
	}
	sequence := append(MouseSGR(0, x, y, false), MouseSGR(0, x, y, true)...)
	return p.app.writeContext(ctx, sequence)
}

// MouseScrollUp sends a scroll-up event at (x, y).
func (p *Page) MouseScrollUp(x, y int) error {
	return p.withDefaultWriteContext(func(ctx context.Context) error {
		return p.MouseScrollUpContext(ctx, x, y)
	})
}

func (p *Page) MouseScrollUpContext(ctx context.Context, x, y int) error {
	if err := p.app.validateCoordinates(x, y); err != nil {
		return err
	}
	return p.app.writeContext(ctx, MouseSGR(64, x, y, false))
}

// MouseScrollDown sends a scroll-down event at (x, y).
func (p *Page) MouseScrollDown(x, y int) error {
	return p.withDefaultWriteContext(func(ctx context.Context) error {
		return p.MouseScrollDownContext(ctx, x, y)
	})
}

func (p *Page) MouseScrollDownContext(ctx context.Context, x, y int) error {
	if err := p.app.validateCoordinates(x, y); err != nil {
		return err
	}
	return p.app.writeContext(ctx, MouseSGR(65, x, y, false))
}

// MouseDown sends a mouse button-down event (0=left, 1=middle, 2=right).
func (p *Page) MouseDown(button, x, y int) error {
	return p.withDefaultWriteContext(func(ctx context.Context) error {
		return p.MouseDownContext(ctx, button, x, y)
	})
}

func (p *Page) MouseDownContext(ctx context.Context, button, x, y int) error {
	p.mouseMu.Lock()
	defer p.mouseMu.Unlock()
	if button < 0 || button > 2 {
		return fmt.Errorf("invalid mouse button %d", button)
	}
	if err := p.app.validateCoordinates(x, y); err != nil {
		return err
	}
	if err := p.app.writeContext(ctx, MouseSGR(button, x, y, false)); err != nil {
		return err
	}
	p.dragBtn = button
	p.dragX = x
	p.dragY = y
	return nil
}

// MouseUp sends a mouse button-up event using the same SGR button code.
func (p *Page) MouseUp(button, x, y int) error {
	return p.withDefaultWriteContext(func(ctx context.Context) error {
		return p.MouseUpContext(ctx, button, x, y)
	})
}

func (p *Page) MouseUpContext(ctx context.Context, button, x, y int) error {
	p.mouseMu.Lock()
	defer p.mouseMu.Unlock()
	if button < 0 || button > 2 {
		return fmt.Errorf("invalid mouse button %d", button)
	}
	if err := p.app.validateCoordinates(x, y); err != nil {
		return err
	}
	if err := p.app.writeContext(ctx, MouseSGR(button, x, y, true)); err != nil {
		return err
	}
	if p.dragBtn == button {
		p.dragBtn = -1
	}
	return nil
}

// MouseMove sends a mouse motion event at (x, y). If a button is held, it
// reports a drag event; otherwise it reports hover motion.
func (p *Page) MouseMove(x, y int) error {
	return p.withDefaultWriteContext(func(ctx context.Context) error {
		return p.MouseMoveContext(ctx, x, y)
	})
}

func (p *Page) MouseMoveContext(ctx context.Context, x, y int) error {
	p.mouseMu.Lock()
	defer p.mouseMu.Unlock()
	if err := p.app.validateCoordinates(x, y); err != nil {
		return err
	}
	if p.dragBtn >= 0 {
		if err := p.app.writeContext(ctx, MouseSGR(p.dragBtn|0x20, x, y, false)); err != nil {
			return err
		}
		p.dragX = x
		p.dragY = y
		return nil
	}
	return p.app.writeContext(ctx, MouseSGRMove(x, y))
}

// EnableMouse is a compatibility no-op. The child TUI controls terminal mouse
// tracking by emitting DECSET sequences; cue-tty injects events through the
// Mouse* methods and does not emulate an interactive host terminal.
//
// Deprecated: call the Mouse* methods to inject events. This method does not
// enable tracking inside the child application.
func (p *Page) EnableMouse() error { return nil }

// DisableMouse is a compatibility no-op; it does not change the child TUI's
// mouse tracking mode.
//
// Deprecated: the child TUI owns its terminal mouse mode.
func (p *Page) DisableMouse() error { return nil }

// WaitFor waits until the screen contains the given text.
func (p *Page) WaitFor(text string, timeout time.Duration) error {
	return p.WaitForContext(context.Background(), text, timeout)
}

// WaitForContext waits for screen text until the timeout or context expires.
func (p *Page) WaitForContext(ctx context.Context, text string, timeout time.Duration) error {
	if timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		t, err := p.Text()
		if err != nil {
			return err
		}
		if strings.Contains(t, text) {
			return nil
		}
		select {
		case <-waitCtx.Done():
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("timeout waiting for %q", text)
		case <-ticker.C:
		}
	}
}

// WaitStable waits until the rendered screen is unchanged for d, with a
// bounded maximum wait to avoid hanging on continuously animated screens.
func (p *Page) WaitStable(d time.Duration) {
	if d <= 0 {
		d = 100 * time.Millisecond
	}
	p.app.waitStable(d)
}
