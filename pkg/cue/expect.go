package cue

import (
	"fmt"
	"regexp"
	"strings"
)

// Expect creates an expectation on a page.
func Expect(p *Page) *Expectation {
	return &Expectation{page: p}
}

// Expectation provides assertion methods.
type Expectation struct {
	page *Page
}

// ToContain asserts that the screen contains substr.
func (e *Expectation) ToContain(substr string) error {
	text, err := e.page.Text()
	if err != nil {
		return err
	}
	if !strings.Contains(text, substr) {
		return fmt.Errorf("expected screen to contain %q, got:\n%s", substr, text)
	}
	return nil
}

// ToMatch asserts that the screen matches a regex pattern.
func (e *Expectation) ToMatch(pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex %q: %w", pattern, err)
	}
	text, err := e.page.Text()
	if err != nil {
		return err
	}
	if !re.MatchString(text) {
		return fmt.Errorf("expected screen to match %q, got:\n%s", pattern, text)
	}
	return nil
}

// ToHaveLine asserts that a specific line equals expected.
func (e *Expectation) ToHaveLine(row int, expected string) error {
	line, err := e.page.Line(row)
	if err != nil {
		return err
	}
	if strings.TrimRight(line, " ") != expected {
		return fmt.Errorf("expected line %d to be %q, got %q", row, expected, line)
	}
	return nil
}

// ToHaveCursorAt asserts the cursor is at (row, col).
func (e *Expectation) ToHaveCursorAt(row, col int) error {
	r, c, err := e.page.Cursor()
	if err != nil {
		return err
	}
	if r != row || c != col {
		return fmt.Errorf("expected cursor at (%d,%d), got (%d,%d)", row, col, r, c)
	}
	return nil
}

// ToNotContain asserts that the screen does NOT contain substr.
func (e *Expectation) ToNotContain(substr string) error {
	text, err := e.page.Text()
	if err != nil {
		return err
	}
	if strings.Contains(text, substr) {
		return fmt.Errorf("expected screen to NOT contain %q", substr)
	}
	return nil
}
