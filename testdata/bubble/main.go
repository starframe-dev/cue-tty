package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	items    []string
	cursor   int
	selected string
	quitting bool
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter":
			m.selected = m.items[m.cursor]
		}
	case tea.MouseMsg:
		if msg.Type == tea.MouseLeft {
			// Map Y position to item index
			// Header is at rows 0-1, items start at row 2
			idx := msg.Y - 2
			if idx >= 0 && idx < len(m.items) {
				m.cursor = idx
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return "Bye!\n"
	}
	s := "Pick an item:\n\n"
	for i, item := range m.items {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		s += fmt.Sprintf("%s %s\n", cursor, item)
	}
	if m.selected != "" {
		s += fmt.Sprintf("\nSelected: %s\n", m.selected)
	}
	s += "\nPress q to quit\n"
	return s
}

func main() {
	m := model{
		items: []string{"Apple", "Banana", "Cherry"},
	}
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
