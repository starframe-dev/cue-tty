package main

import (
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/starframe-dev/warp"
)

func TestServeInspectorStandaloneFallback(t *testing.T) {
	t.Setenv("WARP_HTTP_FD", "")
	t.Setenv("WARP_HTTP_PORT", "0")

	w := warp.New()
	if err := serveInspector(w); err != nil {
		t.Fatalf("serveInspector failed: %v", err)
	}
	defer func() { _ = w.CloseHTTP() }()

	addr := w.HTTPAddr()
	if addr == "" {
		t.Fatal("standalone inspector did not publish its bound address")
	}
	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatalf("healthz request failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestServeInspectorRejectsInvalidInheritedDescriptor(t *testing.T) {
	t.Setenv("WARP_HTTP_FD", "not-a-file-descriptor")
	w := warp.New()
	if err := serveInspector(w); err == nil {
		t.Fatal("serveInspector accepted an invalid inherited descriptor")
	}
}

func TestServeInspectorRejectsClosedInheritedDescriptor(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "closed-listener")
	if err != nil {
		t.Fatal(err)
	}
	fd := file.Fd()
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WARP_HTTP_FD", fmt.Sprint(fd))
	w := warp.New()
	if err := serveInspector(w); err == nil {
		t.Fatal("serveInspector accepted a closed inherited descriptor")
	}
}
