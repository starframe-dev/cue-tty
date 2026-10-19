package cue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/starframe-dev/cue-tty/pkg/internal/privatefs"
)

// Event records an action in the timeline.
type Event struct {
	Timestamp int64  `json:"timestamp"`
	Action    string `json:"action"`
	Detail    string `json:"detail"`
}

// Artifact captures a snapshot of the screen state.
type Artifact struct {
	Name     string  `json:"name"`
	Render   string  `json:"render"`
	ANSI     string  `json:"ansi"`
	Timeline []Event `json:"timeline"`
}

// SaveArtifact captures current state and saves to a directory.
func (p *Page) SaveArtifact(dir string, name string) error {
	if err := validateArtifactName(name); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("refusing non-directory artifact path %q", dir)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}

	snapshot := p.app.Snapshot()
	text := snapshot.Text

	// render.txt
	rpath := filepath.Join(dir, name+".render.txt")
	if err := privatefs.WriteFile(rpath, []byte(text)); err != nil {
		return err
	}

	// state.json
	state := map[string]any{
		"cursor": map[string]int{"row": snapshot.CursorRow, "col": snapshot.CursorCol},
		"text":   text,
	}
	spath := filepath.Join(dir, name+".state.json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := privatefs.WriteFile(spath, data); err != nil {
		return err
	}

	return nil
}

// NewEvent creates an event with current timestamp.
func validateArtifactName(name string) error {
	if name == "" || name == "." || name == ".." || filepath.IsAbs(name) || filepath.Base(name) != name {
		return fmt.Errorf("invalid artifact name %q", name)
	}
	return nil
}

func NewEvent(action, detail string) Event {
	return Event{
		Timestamp: time.Now().UnixMilli(),
		Action:    action,
		Detail:    detail,
	}
}

// WriteTimeline writes a markdown timeline to a file.
func WriteTimeline(dir string, events []Event) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("refusing non-directory timeline path %q", dir)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	var b []byte
	b = append(b, "# Timeline\n\n"...)
	for _, e := range events {
		b = append(b, fmt.Sprintf("- **%d ms**: %s — %s\n", e.Timestamp, e.Action, e.Detail)...)
	}
	return privatefs.WriteFile(filepath.Join(dir, "timeline.md"), b)
}
