package cue

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
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
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	text, err := p.Text()
	if err != nil {
		return err
	}

	// render.txt
	rpath := filepath.Join(dir, name+".render.txt")
	if err := os.WriteFile(rpath, []byte(text), 0o644); err != nil {
		return err
	}

	// state.json
	row, col, err := p.Cursor()
	if err != nil {
		return err
	}
	state := map[string]any{
		"cursor": map[string]int{"row": row, "col": col},
		"text":   text,
	}
	spath := filepath.Join(dir, name+".state.json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(spath, data, 0o644); err != nil {
		return err
	}

	return nil
}

// NewEvent creates an event with current timestamp.
func NewEvent(action, detail string) Event {
	return Event{
		Timestamp: time.Now().UnixMilli(),
		Action:    action,
		Detail:    detail,
	}
}

// WriteTimeline writes a markdown timeline to a file.
func WriteTimeline(dir string, events []Event) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var b []byte
	b = append(b, "# Timeline\n\n"...)
	for _, e := range events {
		b = append(b, fmt.Sprintf("- **%d ms**: %s — %s\n", e.Timestamp, e.Action, e.Detail)...)
	}
	return os.WriteFile(filepath.Join(dir, "timeline.md"), b, 0o644)
}
