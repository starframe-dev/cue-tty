package cue

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/starframe-dev/cue-tty/pkg/ansi"
	"github.com/starframe-dev/cue-tty/pkg/pty"
)

const maxRawLogBytes = 1 << 20

// ScreenSnapshot is a consistent plain-text and cursor view of an app.
type ScreenSnapshot struct {
	Text      string
	CursorRow int
	CursorCol int
}

// TerminalSnapshot includes terminal dimensions and cells from the same frame.
type TerminalSnapshot struct {
	Cols      int
	Rows      int
	Text      string
	CursorRow int
	CursorCol int
	Cells     []ansi.Cell
}

// App represents a running TUI application.
type App struct {
	backend     *pty.Backend
	parser      *ansi.Parser
	screen      *ansi.Screen
	opts        Options
	mu          sync.Mutex
	closeOnce   sync.Once
	closeErr    error
	done        chan struct{}
	timeout     *time.Timer
	timeoutDone chan struct{}
	timeoutStop sync.Once
	rawLog      []byte
}

// Launch starts a TUI application.
func Launch(command string, opts ...Option) (*App, error) {
	o := DefaultOptions()
	for _, fn := range opts {
		fn(&o)
	}
	if !o.EnvReplaced {
		o.Env = append(os.Environ(), o.Env...)
	}

	backend, err := pty.StartWithExtraFiles(command, o.Args, o.Env, o.Dir, o.Width, o.Height, o.ExtraFiles)
	if err != nil {
		return nil, err
	}
	screen := ansi.NewScreen(o.Width, o.Height)
	app := &App{
		backend:     backend,
		parser:      ansi.NewParser(screen),
		screen:      screen,
		opts:        o,
		done:        make(chan struct{}),
		timeoutDone: make(chan struct{}),
	}
	app.parser.SetResponseCallback(func(response []byte) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = app.writeContext(ctx, response)
	})
	if o.Timeout > 0 {
		app.timeout = time.NewTimer(o.Timeout)
		go app.watchTimeout()
	}
	go app.readLoop()
	return app, nil
}

// Page returns the current page for interaction.
func (a *App) Page() *Page { return &Page{app: a, dragBtn: -1} }

// Resize changes terminal dimensions.
func (a *App) Resize(cols, rows int) error {
	if err := pty.ValidateSize(cols, rows); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.backend.Resize(cols, rows); err != nil {
		return err
	}
	if err := a.screen.ResizeChecked(cols, rows); err != nil {
		return fmt.Errorf("resize Portalis screen: %w", err)
	}
	return nil
}

// Close terminates the application and waits for the PTY reader to finish.
func (a *App) Close() error {
	a.closeOnce.Do(func() {
		a.stopTimeout()
		a.closeErr = a.backend.Close()
		<-a.done
	})
	return a.closeErr
}

// ProcessState returns the reaped child process state, or nil before Wait/Close completes.
func (a *App) ProcessState() *os.ProcessState { return a.backend.ProcessState() }

// Wait blocks until the process exits and the PTY reader has stopped.
func (a *App) Wait() error {
	err := a.backend.Wait()
	<-a.done
	a.stopTimeout()
	return err
}

func (a *App) watchTimeout() {
	select {
	case <-a.timeout.C:
		_ = a.Close()
	case <-a.timeoutDone:
	}
}

func (a *App) stopTimeout() {
	a.timeoutStop.Do(func() {
		if a.timeout != nil {
			a.timeout.Stop()
			close(a.timeoutDone)
		}
	})
}

func (a *App) readLoop() {
	defer close(a.done)
	buf := make([]byte, 4096)
	for {
		n, err := a.backend.Read(buf)
		if n > 0 {
			data := buf[:n]
			a.mu.Lock()
			a.appendRawLog(data)
			a.parser.Feed(data)
			a.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

func (a *App) appendRawLog(data []byte) {
	if len(data) >= maxRawLogBytes {
		a.rawLog = append(a.rawLog[:0], data[len(data)-maxRawLogBytes:]...)
		return
	}
	excess := len(a.rawLog) + len(data) - maxRawLogBytes
	if excess > 0 {
		copy(a.rawLog, a.rawLog[excess:])
		a.rawLog = a.rawLog[:len(a.rawLog)-excess]
	}
	a.rawLog = append(a.rawLog, data...)
}

func (a *App) writeContext(ctx context.Context, data []byte) error {
	_, err := a.backend.WriteContext(ctx, data)
	if err != nil && ctx.Err() != nil && a.backend.ProcessState() != nil {
		a.stopTimeout()
	}
	return err
}

// RenderText returns a consistent plain-text screen snapshot.
func (a *App) RenderText() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.screen.RenderText()
}

// Snapshot captures rendered text and cursor position under one lock.
func (a *App) Snapshot() ScreenSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	cursor := a.screen.CurrentCursor()
	return ScreenSnapshot{
		Text:      a.screen.RenderText(),
		CursorRow: cursor.Row,
		CursorCol: cursor.Col,
	}
}

// TerminalSnapshot captures text, dimensions, cursor, and cells under one lock.
func (a *App) TerminalSnapshot() TerminalSnapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	text, cells := a.screen.Snapshot()
	cursor := a.screen.CurrentCursor()
	return TerminalSnapshot{
		Cols: a.screen.Cols(), Rows: a.screen.Rows(), Text: text,
		CursorRow: cursor.Row, CursorCol: cursor.Col, Cells: cells,
	}
}

// CursorPosition returns a consistent cursor snapshot.
func (a *App) CursorPosition() (row, col int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	cursor := a.screen.CurrentCursor()
	return cursor.Row, cursor.Col
}

func (a *App) validateCoordinates(x, y int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if x < 0 || y < 0 || x >= a.screen.Cols() || y >= a.screen.Rows() {
		return fmt.Errorf("mouse coordinates out of range: (%d, %d) for %dx%d screen", x, y, a.screen.Cols(), a.screen.Rows())
	}
	return nil
}

// RawLog returns a copy of the bounded raw PTY output tail.
func (a *App) RawLog() []byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]byte(nil), a.rawLog...)
}

// waitStable waits for screen output to stop changing, up to the given timeout.
func (a *App) waitStable(quietPeriod time.Duration) {
	if quietPeriod <= 0 {
		quietPeriod = 100 * time.Millisecond
	}
	maxWait := quietPeriod * 5
	if maxWait < time.Second {
		maxWait = time.Second
	}
	deadline := time.Now().Add(maxWait)
	last := a.RenderText()
	stableSince := time.Now()
	for time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		current := a.RenderText()
		if current != last {
			last = current
			stableSince = time.Now()
			continue
		}
		if time.Since(stableSince) >= quietPeriod {
			return
		}
	}
}
