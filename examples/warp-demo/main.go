package main

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/starframe-dev/warp"
)

// demoPanel is the main TUI panel for the warp-demo app.
type demoPanel struct {
	items    []string
	selected int
	status   string
}

func newDemoPanel() *demoPanel {
	return &demoPanel{
		items:  []string{"Item 1", "Item 2", "Item 3"},
		status: "Use arrow keys, Enter, or mouse clicks",
	}
}

func (p *demoPanel) View(w, h int) string {
	lines := make([]string, h)
	for i := range lines {
		lines[i] = strings.Repeat(" ", w)
	}

	// Header
	putLine(lines, 0, padCenter("Warp Demo", w), w)
	putLine(lines, 1, strings.Repeat("─", w), w)

	// Items
	putLine(lines, 2, "Items:", w)
	for i, item := range p.items {
		prefix := "  "
		if i == p.selected {
			prefix = "> "
		}
		putLine(lines, 3+i, prefix+item, w)
	}

	// Buttons
	buttonY := 3 + len(p.items) + 1
	putLine(lines, buttonY, "[Add] [Remove] [Select]", w)

	// Status
	statusY := buttonY + 2
	putLine(lines, statusY, padCenter(p.status, w), w)

	return strings.Join(lines, "\n")
}

func putLine(lines []string, idx int, text string, w int) {
	if idx < 0 || idx >= len(lines) {
		return
	}
	if len(text) > w {
		text = text[:w]
	}
	lines[idx] = text + strings.Repeat(" ", w-len(text))
}

func padCenter(s string, w int) string {
	if len(s) >= w {
		return s[:w]
	}
	left := (w - len(s)) / 2
	right := w - len(s) - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

func (p *demoPanel) Update(msg tea.Msg) tea.Cmd {
	switch m := msg.(type) {
	case tea.KeyMsg:
		switch m.String() {
		case "up":
			if p.selected > 0 {
				p.selected--
			}
		case "down":
			if p.selected < len(p.items)-1 {
				p.selected++
			}
		case "enter":
			p.status = fmt.Sprintf("Selected: %s", p.items[p.selected])
		case "a", "A":
			p.addItem()
		case "r", "R":
			p.removeItem()
		case "s", "S":
			p.status = fmt.Sprintf("Selected: %s", p.items[p.selected])
		}

	case tea.MouseMsg:
		// Mouse coordinates are relative to the panel (x, y)
		if m.Button == tea.MouseButtonLeft && m.Action == tea.MouseActionPress {
			p.handleMouse(m.X, m.Y)
		}
	}
	return nil
}

func (p *demoPanel) addItem() {
	p.items = append(p.items, fmt.Sprintf("Item %d", len(p.items)+1))
	p.status = fmt.Sprintf("Added %d items total", len(p.items))
}

func (p *demoPanel) removeItem() {
	if len(p.items) == 0 {
		p.status = "Nothing to remove"
		return
	}
	removed := p.items[p.selected]
	p.items = append(p.items[:p.selected], p.items[p.selected+1:]...)
	if p.selected >= len(p.items) && p.selected > 0 {
		p.selected--
	}
	p.status = fmt.Sprintf("Removed: %s", removed)
}

func (p *demoPanel) handleMouse(x, y int) {
	// Items start at row 3, each takes 1 row
	itemY := 3
	if y >= itemY && y < itemY+len(p.items) {
		p.selected = y - itemY
		p.status = fmt.Sprintf("Selected: %s", p.items[p.selected])
		return
	}

	// Buttons row
	buttonY := itemY + len(p.items) + 1
	if y == buttonY {
		if x >= 0 && x < 5 {
			p.addItem()
		} else if x >= 6 && x < 14 {
			p.removeItem()
		} else if x >= 15 && x < 23 {
			p.status = fmt.Sprintf("Selected: %s", p.items[p.selected])
		}
	}
}

// Elements exposes the UI structure for HTTP /elements endpoint.
func (p *demoPanel) Elements(width, height int) []warp.Element {
	itemsY := 3
	buttonsY := itemsY + len(p.items) + 1

	elems := []warp.Element{
		{
			Role:   "header",
			Name:   "Warp Demo",
			Bounds: warp.Bounds{X: 0, Y: 0, W: width, H: 1},
		},
	}

	// List items
	for i, item := range p.items {
		prefix := "  "
		if i == p.selected {
			prefix = "> "
		}
		name := prefix + item
		action := fmt.Sprintf("select-item-%d", i)
		elems = append(elems, warp.Element{
			Role:   "listitem",
			Name:   name,
			Action: action,
			Bounds: warp.Bounds{X: 0, Y: itemsY + i, W: len(name), H: 1},
		})
	}

	// Buttons
	buttons := []struct {
		name, action string
		x, w         int
	}{
		{"Add", "add", 1, 3},
		{"Remove", "remove", 7, 6},
		{"Select", "select", 16, 6},
	}
	for _, b := range buttons {
		elems = append(elems, warp.Element{
			Role:   "button",
			Name:   b.name,
			Action: b.action,
			Bounds: warp.Bounds{X: b.x, Y: buttonsY, W: b.w, H: 1},
		})
	}

	return elems
}

func main() {
	p := newDemoPanel()

	w := warp.New()
	w.SetRoot(p)

	// Start HTTP element tree server
	addr := "127.0.0.1:0"
	if port := os.Getenv("WARP_HTTP_PORT"); port != "" {
		addr = "127.0.0.1:" + port
	}
	if err := w.ServeHTTPWithOptions(addr, warp.InspectorOptions{BearerToken: os.Getenv("WARP_HTTP_TOKEN")}); err != nil {
		panic(err)
	}

	if err := w.Run(); err != nil {
		panic(err)
	}
}
