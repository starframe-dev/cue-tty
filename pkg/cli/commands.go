package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/starframe-dev/cue-tty/pkg/internal/privatefs"
)

// CommandHandler handles a single CLI command.
type CommandHandler func(session *Session, args []string) (string, error)

// Commands returns the map of command names to handlers.
func Commands(sm *SessionManager) map[string]CommandHandler {
	return map[string]CommandHandler{
		"launch":  cmdLaunch,
		"press":   cmdPress,
		"type":    cmdType,
		"text":    cmdText,
		"lines":   cmdLines,
		"line":    cmdLine,
		"cursor":  cmdCursor,
		"click":   cmdClick,
		"scroll":  cmdScroll,
		"mouse":   cmdMouse,
		"render":  cmdRender,
		"node":    cmdNode,
		"save":    cmdSave,
		"resize":  cmdResize,
		"wait":    cmdWait,
		"waitfor": cmdWaitFor,
		"expect":  cmdExpect,
		"close":   cmdCloseSession,
		"show":    makeCmdShow(sm),
		"list":    makeCmdList(sm),
		"exit":    cmdExit,
		"quit":    cmdExit,
		"help":    cmdHelp,
	}
}

func cmdLaunch(session *Session, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: launch <command> [args...]")
	}

	command := args[0]
	cmdArgs := args[1:]

	if err := session.Launch(command, cmdArgs...); err != nil {
		return "", err
	}

	return fmt.Sprintf("Launched %q in session %q", command, session.Name), nil
}

func cmdPress(session *Session, args []string) (string, error) {
	if session.App == nil {
		return "", fmt.Errorf("no app running in session %q", session.Name)
	}
	if len(args) == 0 {
		return "", fmt.Errorf("usage: press <key>")
	}

	key := strings.Join(args, " ")
	if err := session.Page.PressContext(session.context(), key); err != nil {
		return "", err
	}
	return fmt.Sprintf("Pressed %q", key), nil
}

func cmdType(session *Session, args []string) (string, error) {
	if session.App == nil {
		return "", fmt.Errorf("no app running in session %q", session.Name)
	}
	if len(args) == 0 {
		return "", fmt.Errorf("usage: type <text>")
	}

	text := strings.Join(args, " ")
	if err := session.Page.TypeContext(session.context(), text); err != nil {
		return "", err
	}
	return fmt.Sprintf("Typed %q", text), nil
}

func cmdText(session *Session, args []string) (string, error) {
	if session.App == nil {
		return "", fmt.Errorf("no app running in session %q", session.Name)
	}
	text, err := session.Page.Text()
	if err != nil {
		return "", err
	}
	return text, nil
}

func cmdLines(session *Session, args []string) (string, error) {
	if session.App == nil {
		return "", fmt.Errorf("no app running in session %q", session.Name)
	}
	lines, err := session.Page.Lines()
	if err != nil {
		return "", err
	}

	var b strings.Builder
	for i, line := range lines {
		display := truncateDisplayWidth(line, 80)
		b.WriteString(fmt.Sprintf("[%2d] %s\n", i, display))
	}
	return b.String(), nil
}

func cmdLine(session *Session, args []string) (string, error) {
	if session.App == nil {
		return "", fmt.Errorf("no app running in session %q", session.Name)
	}
	if len(args) == 0 {
		return "", fmt.Errorf("usage: line <n>")
	}

	n, err := strconv.Atoi(args[0])
	if err != nil {
		return "", fmt.Errorf("invalid line number: %q", args[0])
	}

	line, err := session.Page.Line(n)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("[%2d] %s", n, line), nil
}

func cmdCursor(session *Session, args []string) (string, error) {
	if session.App == nil {
		return "", fmt.Errorf("no app running in session %q", session.Name)
	}
	row, col, err := session.Page.Cursor()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("Cursor at (%d, %d)", row, col), nil
}

func cmdClick(session *Session, args []string) (string, error) {
	if session.App == nil {
		return "", fmt.Errorf("no app running in session %q", session.Name)
	}
	if len(args) < 1 {
		return "", fmt.Errorf("usage: click <x> <y> | click <role>:<name>")
	}

	// Check if first arg contains ':' — element lookup mode
	if strings.Contains(args[0], ":") {
		if session.HTTPAddr == "" {
			return "", fmt.Errorf("element lookup requires HTTP element tree (not available)")
		}

		parts := strings.SplitN(args[0], ":", 2)
		role := parts[0]
		name := ""
		if len(parts) > 1 {
			name = parts[1]
		}

		elems, err := FetchElementsWithToken(session.HTTPAddr, session.httpToken)
		if err != nil {
			return "", fmt.Errorf("fetch elements: %w", err)
		}

		el, found := FindElement(elems, role, name, "")
		if !found {
			return "", fmt.Errorf("element not found: %s:%s", role, name)
		}

		x, y := el.Bounds.Center()
		if err := session.Page.MouseClickContext(session.context(), x, y); err != nil {
			return "", err
		}
		return fmt.Sprintf("Clicked <%s> %q at (%d, %d)", el.Role, el.Name, x, y), nil
	}

	// Raw coordinate mode
	if len(args) < 2 {
		return "", fmt.Errorf("usage: click <x> <y> | click <role>:<name>")
	}

	x, err := strconv.Atoi(args[0])
	if err != nil {
		return "", fmt.Errorf("invalid x: %q", args[0])
	}
	y, err := strconv.Atoi(args[1])
	if err != nil {
		return "", fmt.Errorf("invalid y: %q", args[1])
	}

	if err := session.Page.MouseClickContext(session.context(), x, y); err != nil {
		return "", err
	}
	return fmt.Sprintf("Clicked at (%d, %d)", x, y), nil
}

func cmdScroll(session *Session, args []string) (string, error) {
	if session.App == nil {
		return "", fmt.Errorf("no app running in session %q", session.Name)
	}
	if len(args) < 3 {
		return "", fmt.Errorf("usage: scroll <up|down> <x> <y>")
	}

	direction := args[0]
	x, err := strconv.Atoi(args[1])
	if err != nil {
		return "", fmt.Errorf("invalid x: %q", args[1])
	}
	y, err := strconv.Atoi(args[2])
	if err != nil {
		return "", fmt.Errorf("invalid y: %q", args[2])
	}

	switch direction {
	case "up":
		if err := session.Page.MouseScrollUpContext(session.context(), x, y); err != nil {
			return "", err
		}
	case "down":
		if err := session.Page.MouseScrollDownContext(session.context(), x, y); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("direction must be 'up' or 'down', got %q", direction)
	}

	return fmt.Sprintf("Scrolled %s at (%d, %d)", direction, x, y), nil
}

func cmdMouse(session *Session, args []string) (string, error) {
	if session.App == nil {
		return "", fmt.Errorf("no app running in session %q", session.Name)
	}
	if len(args) < 3 {
		return "", fmt.Errorf("usage: mouse <down|up> <x> <y>")
	}

	action := args[0]
	x, err := strconv.Atoi(args[1])
	if err != nil {
		return "", fmt.Errorf("invalid x: %q", args[1])
	}
	y, err := strconv.Atoi(args[2])
	if err != nil {
		return "", fmt.Errorf("invalid y: %q", args[2])
	}

	switch action {
	case "down":
		if err := session.Page.MouseDownContext(session.context(), 0, x, y); err != nil {
			return "", err
		}
	case "up":
		if err := session.Page.MouseUpContext(session.context(), 0, x, y); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("action must be 'down' or 'up', got %q", action)
	}

	return fmt.Sprintf("Mouse %s at (%d, %d)", action, x, y), nil
}

func cmdRender(session *Session, args []string) (string, error) {
	if session.App == nil {
		return "", fmt.Errorf("no app running in session %q", session.Name)
	}
	text, err := session.Page.Text()
	if err != nil {
		return "", err
	}
	return text, nil
}

func cmdNode(session *Session, args []string) (string, error) {
	if session.App == nil {
		return "", fmt.Errorf("no app running in session %q", session.Name)
	}

	// If HTTP element tree is available, show that instead
	if session.HTTPAddr != "" {
		elems, err := FetchElementsWithToken(session.HTTPAddr, session.httpToken)
		if err != nil {
			return "", fmt.Errorf("fetch element tree: %w", err)
		}
		return "Element tree:\n" + RenderElementTree(elems, ""), nil
	}

	nt, err := BuildNodeTree(session.Page)
	if err != nil {
		return "", err
	}
	return nt.Render(), nil
}

func cmdSave(session *Session, args []string) (string, error) {
	if !validSessionName(session.Name) {
		return "", fmt.Errorf("invalid session name %q", session.Name)
	}
	if session.App == nil {
		return "", fmt.Errorf("no app running in session %q", session.Name)
	}
	name := "snapshot"
	if len(args) > 0 {
		name = args[0]
	}
	if !validSessionName(name) {
		return "", fmt.Errorf("invalid artifact name %q", name)
	}

	baseDir := session.WorkDir
	if baseDir == "" {
		baseDir = "."
	}
	artifactsRoot := filepath.Join(baseDir, "cuetty-artifacts")
	sessionDir := filepath.Join(artifactsRoot, session.Name)
	dir := filepath.Join(sessionDir, name)
	for _, privateDir := range []string{artifactsRoot, sessionDir, dir} {
		if err := ensurePrivateDir(privateDir); err != nil {
			return "", err
		}
	}

	// Save render.txt and cursor from one screen snapshot.
	snapshot := session.App.Snapshot()
	text := snapshot.Text
	if err := privatefs.WriteFile(filepath.Join(dir, "render.txt"), []byte(text)); err != nil {
		return "", err
	}

	// Save state.json
	state := fmt.Sprintf(`{
  "session": %q,
  "cursor": {"row": %d, "col": %d},
  "text": %q
}`, session.Name, snapshot.CursorRow, snapshot.CursorCol, text)
	if err := privatefs.WriteFile(filepath.Join(dir, "state.json"), []byte(state)); err != nil {
		return "", err
	}

	return fmt.Sprintf("Saved to %s/", dir), nil
}

func ensurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("refusing non-directory artifact path %q", path)
	}
	return os.Chmod(path, 0700)
}

func cmdResize(session *Session, args []string) (string, error) {
	if session.App == nil {
		return "", fmt.Errorf("no app running in session %q", session.Name)
	}
	if len(args) < 2 {
		return "", fmt.Errorf("usage: resize <cols> <rows>")
	}

	cols, err := strconv.Atoi(args[0])
	if err != nil {
		return "", fmt.Errorf("invalid cols: %q", args[0])
	}
	rows, err := strconv.Atoi(args[1])
	if err != nil {
		return "", fmt.Errorf("invalid rows: %q", args[1])
	}

	if err := session.App.Resize(cols, rows); err != nil {
		return "", err
	}
	return fmt.Sprintf("Resized to %dx%d", cols, rows), nil
}

func cmdWait(session *Session, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: wait <ms>")
	}

	ms, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid ms: %q", args[0])
	}
	maxMilliseconds := int64(maxCommandDuration / time.Millisecond)
	if ms < 0 || ms > maxMilliseconds {
		return "", fmt.Errorf("wait duration must be between 0 and %d ms", maxMilliseconds)
	}
	timer := time.NewTimer(time.Duration(ms) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-session.context().Done():
		return "", session.context().Err()
	case <-timer.C:
		return fmt.Sprintf("Waited %dms", ms), nil
	}
}

func cmdWaitFor(session *Session, args []string) (string, error) {
	if session.App == nil {
		return "", fmt.Errorf("no app running in session %q", session.Name)
	}
	if len(args) == 0 {
		return "", fmt.Errorf("usage: waitfor <text> [timeout-ms]")
	}
	timeout := 5 * time.Second
	textArgs := args
	if len(args) > 1 {
		last := args[len(args)-1]
		milliseconds, err := strconv.ParseInt(last, 10, 64)
		if err == nil {
			maxMilliseconds := int64(maxCommandDuration / time.Millisecond)
			if milliseconds <= 0 || milliseconds > maxMilliseconds {
				return "", fmt.Errorf("timeout must be between 1 and %d ms", maxMilliseconds)
			}
			timeout = time.Duration(milliseconds) * time.Millisecond
			textArgs = args[:len(args)-1]
		} else if isIntegerToken(last) {
			return "", fmt.Errorf("invalid timeout: %q", last)
		}
	}
	text := strings.Join(textArgs, " ")
	if err := session.Page.WaitForContext(session.context(), text, timeout); err != nil {
		return "", err
	}
	return fmt.Sprintf("Found %q", text), nil
}

func isIntegerToken(value string) bool {
	if value == "" {
		return false
	}
	if value[0] == '+' || value[0] == '-' {
		value = value[1:]
	}
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func cmdExpect(session *Session, args []string) (string, error) {
	if session.App == nil {
		return "", fmt.Errorf("no app running in session %q", session.Name)
	}
	if len(args) < 2 {
		return "", fmt.Errorf("usage: expect <contain|not> <text>")
	}

	mode := args[0]
	text := strings.Join(args[1:], " ")

	screenText, err := session.Page.Text()
	if err != nil {
		return "", err
	}

	switch mode {
	case "contain":
		if strings.Contains(screenText, text) {
			return fmt.Sprintf("✅ Found %q", text), nil
		}
		return "", fmt.Errorf("❌ not found %q", text)
	case "not":
		if strings.Contains(screenText, text) {
			return "", fmt.Errorf("❌ found %q (expected not to)", text)
		}
		return fmt.Sprintf("✅ Not found %q (as expected)", text), nil
	default:
		return "", fmt.Errorf("mode must be 'contain' or 'not', got %q", mode)
	}
}

func cmdCloseSession(session *Session, args []string) (string, error) {
	if session.App == nil {
		return "No app running", nil
	}
	if err := session.Close(); err != nil {
		return "", err
	}
	session.App = nil
	session.Page = nil
	return "Session closed", nil
}

func cmdExit(session *Session, args []string) (string, error) {
	if session.App != nil {
		if err := session.Close(); err != nil {
			return "", err
		}
		session.App = nil
		session.Page = nil
		session.HTTPAddr = ""
	}
	return "Session closed", nil
}

func cmdHelp(session *Session, args []string) (string, error) {
	return `Commands:
  launch <command> [args...]  — launch a TUI application
  press <key>                 — press a key (Enter, Ctrl+C, F1, Up, etc.)
  type <text>                 — type text character by character
  text                        — show plain text of the screen
  lines                       — show all lines with numbers
  line <n>                    — show a specific line
  cursor                      — show cursor position
  click <x> <y>              — mouse click at (x, y)
  click <role>:<name>        — click element by role:name (requires HTTP element tree)
  scroll <up|down> <x> <y>   — scroll at (x, y)
  mouse <down|up> <x> <y>    — mouse button down/up
  render                      — show screen text
  node                        — show node tree (screen structure) or element tree (if HTTP available)
  save [name]                 — save artifacts (render.txt + state.json)
  resize <cols> <rows>        — resize terminal
  wait <ms>                   — wait 0..60000 milliseconds
  waitfor <text> [timeout-ms] — wait up to 60000ms for screen text
  expect <contain|not> <text> — assert text on screen
  close                       — close the current app
  show                        — show dashboard of all sessions
  list                        — list all active sessions
  exit / quit                 — exit the REPL
  help                        — show this help
`, nil
}

// makeCmdShow returns a CommandHandler that shows all sessions.
func makeCmdShow(sm *SessionManager) CommandHandler {
	return func(session *Session, args []string) (string, error) {
		sessions := sm.Snapshots()
		if len(sessions) == 0 {
			return "No active sessions", nil
		}

		var b strings.Builder
		b.WriteString(fmt.Sprintf("Active sessions (%d):\n", len(sessions)))
		b.WriteString("──────────────────────────────────────────────────\n")

		for _, snapshot := range sessions {
			status := "● running"
			if !snapshot.Running {
				status = "○ idle"
			}
			b.WriteString(fmt.Sprintf("  %-20s %s\n", snapshot.Name, status))

			if snapshot.Running {
				lines := strings.Split(snapshot.Text, "\n")
				maxLines := 5
				if len(lines) < maxLines {
					maxLines = len(lines)
				}
				for i := 0; i < maxLines; i++ {
					b.WriteString(fmt.Sprintf("    │ %s\n", truncateDisplayWidth(lines[i], 60)))
				}
				if len(lines) > maxLines {
					b.WriteString(fmt.Sprintf("    │ ... (%d more lines)\n", len(lines)-maxLines))
				}
			}
			b.WriteString("\n")
		}

		return b.String(), nil
	}
}

// makeCmdList returns a CommandHandler that lists all sessions.
func makeCmdList(sm *SessionManager) CommandHandler {
	return func(session *Session, args []string) (string, error) {
		sessions := sm.Snapshots()
		if len(sessions) == 0 {
			return "No active sessions", nil
		}

		var b strings.Builder
		for _, snapshot := range sessions {
			status := "running"
			if !snapshot.Running {
				status = "idle"
			}
			b.WriteString(fmt.Sprintf("  %s  (%s)\n", snapshot.Name, status))
		}

		return b.String(), nil
	}
}
