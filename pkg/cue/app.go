package cue

import (
	"bytes"
	"os"
	"sync"
	"time"

	"github.com/starframe-dev/cue-tty/pkg/ansi"
	"github.com/starframe-dev/cue-tty/pkg/pty"
)

// App represents a running TUI application.
type App struct {
	backend    *pty.Backend
	parser     *ansi.Parser
	screen     *ansi.Screen
	opts       Options
	mu         sync.Mutex
	done       chan struct{}
	stopReader chan struct{}
	rawLog     []byte // debug: raw PTY output
}

// Launch starts a TUI application.
func Launch(command string, opts ...Option) (*App, error) {
	o := DefaultOptions()
	for _, fn := range opts {
		fn(&o)
	}

	// Merge with the current process environment by default so that env vars
	// set by the application under test (e.g. WARP_HTTP_PORT) remain visible.
	if !o.EnvReplaced {
		o.Env = append(os.Environ(), o.Env...)
	}

	backend, err := pty.Start(command, o.Args, o.Env, o.Dir, o.Width, o.Height)
	if err != nil {
		return nil, err
	}

	screen := ansi.NewScreen(o.Width, o.Height)
	parser := ansi.NewParser(screen)

	app := &App{
		backend:    backend,
		parser:     parser,
		screen:     screen,
		opts:       o,
		done:       make(chan struct{}),
		stopReader: make(chan struct{}),
	}

	go app.readLoop()

	return app, nil
}

// Page returns the current page for interaction.
func (a *App) Page() *Page {
	return &Page{app: a}
}

// Resize changes terminal dimensions.
func (a *App) Resize(cols, rows int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.screen.Resize(cols, rows)
	return a.backend.Resize(cols, rows)
}

// Close kills the application.
func (a *App) Close() error {
	close(a.stopReader)
	return a.backend.Close()
}

// Wait blocks until the process exits.
func (a *App) Wait() error {
	return a.backend.Wait()
}

func (a *App) readLoop() {
	dataCh := make(chan []byte)
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := a.backend.Read(buf)
			if err != nil {
				close(dataCh)
				return
			}
			if n > 0 {
				cp := make([]byte, n)
				copy(cp, buf[:n])
				dataCh <- cp
			}
		}
	}()

	for {
		select {
		case <-a.stopReader:
			return
		case data, ok := <-dataCh:
			if !ok {
				return
			}
			a.mu.Lock()
			a.rawLog = append(a.rawLog, data...)
			a.parser.Feed(data)
			a.mu.Unlock()

			// Auto-respond to common terminal queries so apps don't hang
			if bytes.Contains(data, []byte("\x1b[6n")) {
				// Cursor position report: ESC[row;colR
				_, _ = a.backend.Write([]byte("\x1b[1;1R"))
			}
			if bytes.Contains(data, []byte("\x1b[5n")) {
				// Status report: ESC[0n (OK)
				_, _ = a.backend.Write([]byte("\x1b[0n"))
			}
		}
	}
}

func (a *App) write(data []byte) error {
	_, err := a.backend.Write(data)
	return err
}

// Screen returns the current screen state (locked).
func (a *App) screenState() *ansi.Screen {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.screen
}

// RenderText returns the screen as plain text, locked.
func (a *App) RenderText() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.screen.RenderText()
}

// RawLog returns the raw bytes captured from the PTY (for debugging).
func (a *App) RawLog() []byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]byte(nil), a.rawLog...)
}

// waitStable waits until no new output arrives for the given duration.
func (a *App) waitStable(d time.Duration) {
	if d == 0 {
		d = 50 * time.Millisecond
	}
	time.Sleep(d)
}
