package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RemoteElement mirrors warp.Element for HTTP deserialization.
type RemoteElement struct {
	Role     string          `json:"role"`
	Name     string          `json:"name"`
	Action   string          `json:"action,omitempty"`
	Bounds   RemoteBounds    `json:"bounds"`
	Children []RemoteElement `json:"children,omitempty"`
}

// RemoteBounds mirrors warp.Bounds.
type RemoteBounds struct {
	X int `json:"x"`
	Y int `json:"y"`
	W int `json:"w"`
	H int `json:"h"`
}

// Center returns the center cell of the bounds.
func (b RemoteBounds) Center() (int, int) {
	return b.X + b.W/2, b.Y + b.H/2
}

// FetchElements fetches the element tree from the app's HTTP server.
func FetchElements(httpAddr string) ([]RemoteElement, error) {
	if httpAddr == "" {
		return nil, fmt.Errorf("no HTTP address configured")
	}

	url := httpAddr + "/elements"
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "http://127.0.0.1:" + url + "/elements"
	}
	// Replace localhost with 127.0.0.1 to avoid IPv6 issues
	url = strings.Replace(url, "localhost", "127.0.0.1", 1)

	var body []byte
	var resp *http.Response
	var err error

	for i := 0; i < 50; i++ {
		resp, err = http.Get(url)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		return nil, fmt.Errorf("fetch elements: %w", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var elements []RemoteElement
	if err := json.Unmarshal(body, &elements); err != nil {
		return nil, fmt.Errorf("parse elements: %w", err)
	}

	return elements, nil
}

// FindElement recursively searches elements for the first matching role/name/action.
func FindElement(elems []RemoteElement, role, name, action string) (RemoteElement, bool) {
	for _, el := range elems {
		if (role == "" || el.Role == role) &&
			(name == "" || el.Name == name) &&
			(action == "" || el.Action == action) {
			return el, true
		}
		if found, ok := FindElement(el.Children, role, name, action); ok {
			return found, true
		}
	}
	return RemoteElement{}, false
}

// RenderElementTree renders the element tree as a string.
func RenderElementTree(elems []RemoteElement, indent string) string {
	var b strings.Builder
	for i, el := range elems {
		prefix := "├──"
		if i == len(elems)-1 {
			prefix = "└──"
		}

		action := ""
		if el.Action != "" {
			action = fmt.Sprintf(" [action=%q]", el.Action)
		}
		bounds := fmt.Sprintf(" (%d,%d %dx%d)", el.Bounds.X, el.Bounds.Y, el.Bounds.W, el.Bounds.H)
		b.WriteString(fmt.Sprintf("%s%s <%s> %q%s%s\n",
			indent, prefix, el.Role, el.Name, bounds, action))

		if len(el.Children) > 0 {
			childIndent := indent + "    "
			if i < len(elems)-1 {
				childIndent = indent + "│   "
			}
			b.WriteString(RenderElementTree(el.Children, childIndent))
		}
	}
	return b.String()
}
