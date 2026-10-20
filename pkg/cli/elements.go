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
	return FetchElementsWithToken(httpAddr, "")
}

// FetchElementsWithToken fetches an element tree using an optional bearer token.
func FetchElementsWithToken(httpAddr, token string) ([]RemoteElement, error) {
	elements, _, err := fetchElementsWithToken(httpAddr, token)
	return elements, err
}

const (
	inspectorResponseHeaderTimeout = 100 * time.Millisecond
	inspectorRequestTimeout        = 500 * time.Millisecond
)

// fetchElementsWithToken reports whether the endpoint returned an HTTP response,
// including error status codes and malformed response bodies.
func fetchElementsWithToken(httpAddr, token string) ([]RemoteElement, bool, error) {
	if httpAddr == "" {
		return nil, false, fmt.Errorf("no HTTP address configured")
	}

	baseURL := strings.TrimRight(httpAddr, "/")
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}
	url := baseURL + "/elements"
	// Resolve localhost explicitly to IPv4 for local Warp servers.
	url = strings.Replace(url, "localhost", "127.0.0.1", 1)

	roundTripper := http.DefaultTransport
	if baseTransport, ok := roundTripper.(*http.Transport); ok {
		transport := baseTransport.Clone()
		transport.ResponseHeaderTimeout = inspectorResponseHeaderTimeout
		defer transport.CloseIdleConnections()
		roundTripper = transport
	}
	client := &http.Client{Timeout: inspectorRequestTimeout, Transport: roundTripper}
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("create elements request: %w", err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(request)
	receivedHTTPResponse := resp != nil
	if err != nil {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		return nil, receivedHTTPResponse, fmt.Errorf("fetch elements: %w", err)
	}
	if resp == nil {
		return nil, false, fmt.Errorf("fetch elements: received no HTTP response")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, true, fmt.Errorf("fetch elements: HTTP status %s", resp.Status)
	}

	const maxResponseBytes = 1 << 20
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, true, fmt.Errorf("read response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return nil, true, fmt.Errorf("elements response exceeds %d bytes", maxResponseBytes)
	}

	var elements []RemoteElement
	if err := json.Unmarshal(body, &elements); err != nil {
		return nil, true, fmt.Errorf("parse elements: %w", err)
	}

	return elements, true, nil
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
