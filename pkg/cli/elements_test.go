package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchElements(t *testing.T) {
	expected := []RemoteElement{
		{
			Role: "header",
			Name: "Test App",
			Bounds: RemoteBounds{X: 0, Y: 0, W: 80, H: 1},
			Children: []RemoteElement{
				{
					Role: "button",
					Name: "Add",
					Action: "add",
					Bounds: RemoteBounds{X: 0, Y: 2, W: 5, H: 1},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/elements" {
			t.Errorf("expected path /elements, got %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(expected); err != nil {
			t.Fatalf("encode response: %v", err)
		}
	}))
	defer server.Close()

	elems, err := FetchElements(server.URL)
	if err != nil {
		t.Fatalf("FetchElements: %v", err)
	}

	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	if elems[0].Role != "header" || elems[0].Name != "Test App" {
		t.Errorf("unexpected root element: %+v", elems[0])
	}
	if len(elems[0].Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(elems[0].Children))
	}
	if elems[0].Children[0].Name != "Add" {
		t.Errorf("unexpected child name: %q", elems[0].Children[0].Name)
	}
}

func TestFindElement(t *testing.T) {
	elems := []RemoteElement{
		{
			Role: "header",
			Name: "App",
			Children: []RemoteElement{
				{Role: "button", Name: "Add", Action: "add", Bounds: RemoteBounds{X: 0, Y: 2, W: 5, H: 1}},
				{Role: "button", Name: "Remove", Action: "remove", Bounds: RemoteBounds{X: 6, Y: 2, W: 8, H: 1}},
			},
		},
	}

	t.Run("find by role", func(t *testing.T) {
		el, found := FindElement(elems, "button", "", "")
		if !found {
			t.Fatal("expected to find button")
		}
		if el.Name != "Add" {
			t.Errorf("expected first button, got %q", el.Name)
		}
	})

	t.Run("find by role and name", func(t *testing.T) {
		el, found := FindElement(elems, "button", "Remove", "")
		if !found {
			t.Fatal("expected to find Remove button")
		}
		if el.Name != "Remove" {
			t.Errorf("expected Remove, got %q", el.Name)
		}
	})

	t.Run("find by action", func(t *testing.T) {
		el, found := FindElement(elems, "", "", "add")
		if !found {
			t.Fatal("expected to find add action")
		}
		if el.Name != "Add" {
			t.Errorf("expected Add, got %q", el.Name)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, found := FindElement(elems, "button", "Delete", "")
		if found {
			t.Error("expected not to find Delete button")
		}
	})
}

func TestRenderElementTree(t *testing.T) {
	elems := []RemoteElement{
		{
			Role: "header",
			Name: "App",
			Bounds: RemoteBounds{X: 0, Y: 0, W: 80, H: 1},
			Children: []RemoteElement{
				{
					Role:   "button",
					Name:   "Add",
					Action: "add",
					Bounds: RemoteBounds{X: 0, Y: 1, W: 5, H: 1},
				},
			},
		},
	}

	rendered := RenderElementTree(elems, "")
	if !strings.Contains(rendered, "<header> \"App\"") {
		t.Errorf("expected header in render, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "<button> \"Add\"") {
		t.Errorf("expected button in render, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "[action=\"add\"]") {
		t.Errorf("expected action in render, got:\n%s", rendered)
	}
}

func TestRemoteBoundsCenter(t *testing.T) {
	b := RemoteBounds{X: 10, Y: 20, W: 6, H: 4}
	x, y := b.Center()
	if x != 13 || y != 22 {
		t.Errorf("Center() = (%d, %d), want (13, 22)", x, y)
	}
}
