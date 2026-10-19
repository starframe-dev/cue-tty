package cli

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestDaemonBoundsRequestBody(t *testing.T) {
	d := NewDaemon()
	body := `{"session":"` + strings.Repeat("x", 1<<20) + `","command":"list"}`
	req := httptest.NewRequest("POST", "/cmd", strings.NewReader(body))
	response := httptest.NewRecorder()
	d.handleCommand(response, req)
	if response.Body.Len() == 0 {
		t.Fatal("expected an error response for oversized request")
	}
	if len(d.sm.List()) != 0 {
		t.Fatal("oversized request should not create a session")
	}
}

func TestDaemonRejectsSessionPathTraversal(t *testing.T) {
	d := NewDaemon()
	response := callDaemonCommand(t, d, DaemonRequest{
		Session: "../escape",
		Command: "launch",
		Args:    []string{"sleep", "1"},
	})
	if response.Error == "" {
		t.Fatal("expected path traversal session name to be rejected")
	}
	if got := d.sm.List(); len(got) != 0 {
		t.Fatalf("invalid session was created: %v", got)
	}
}

func TestDaemonLockExcludesConcurrentInstances(t *testing.T) {
	path := filepath.Join(t.TempDir(), "daemon.lock")
	unlock, err := acquireDaemonLock(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := acquireDaemonLock(path); err == nil {
		t.Fatal("second daemon lock unexpectedly acquired")
	}
	unlock()
	unlockAgain, err := acquireDaemonLock(path)
	if err != nil {
		t.Fatalf("lock could not be reacquired after release: %v", err)
	}
	unlockAgain()
}

func TestSocketDirectoryIsPrivate(t *testing.T) {
	info, err := os.Stat(filepath.Dir(SocketPath()))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("socket directory permissions are too broad: %o", info.Mode().Perm())
	}
}

func TestDaemonRefusesToRemoveRegularSocketPathTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-socket")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := NewDaemon()
	d.socketPath = path
	if err := d.Run(); err == nil {
		t.Fatal("expected daemon to refuse a regular file at the socket path")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "keep" {
		t.Fatalf("socket path target was modified: data=%q err=%v", data, err)
	}
}

func TestDaemonPassesWorkDirToRelativeLaunch(t *testing.T) {
	d := NewDaemon()
	defer d.sm.CloseAll()

	workDir := t.TempDir()
	program := filepath.Join(workDir, "relative-app")
	if err := os.WriteFile(program, []byte("#!/bin/sh\nprintf 'workdir-ok\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	response := callDaemonCommand(t, d, DaemonRequest{
		Session: "workdir-session",
		Command: "launch",
		Args:    []string{"./relative-app"},
		WorkDir: workDir,
	})
	if response.Error != "" {
		t.Fatalf("launch failed: %s", response.Error)
	}
	if _, ok := d.sm.Get("workdir-session"); !ok {
		t.Fatal("daemon did not retain the launched session")
	}
	response = callDaemonCommand(t, d, DaemonRequest{
		Session: "workdir-session",
		Command: "waitfor",
		Args:    []string{"workdir-ok", "2000"},
		WorkDir: workDir,
	})
	if response.Error != "" {
		t.Fatalf("relative executable did not produce output: %s", response.Error)
	}
}

func callDaemonCommand(t *testing.T, d *Daemon, request DaemonRequest) DaemonResponse {
	t.Helper()
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/cmd", bytes.NewReader(payload))
	response := httptest.NewRecorder()
	d.handleCommand(response, req)
	var result DaemonResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestIndependentSessionsCanLaunchConcurrently(t *testing.T) {
	d := NewDaemon()
	defer d.sm.CloseAll()

	var wg sync.WaitGroup
	errs := make(chan string, 2)
	for _, name := range []string{"session-one", "session-two"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			response := callDaemonCommand(t, d, DaemonRequest{
				Session: name,
				Command: "launch",
				Args:    []string{"sleep", "5"},
			})
			errs <- response.Error
		}(name)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != "" {
			t.Fatalf("independent session launch failed: %s", err)
		}
	}
}

func TestConcurrentCommandsSerializePerSession(t *testing.T) {
	d := NewDaemon()
	defer d.sm.CloseAll()

	var wg sync.WaitGroup
	responses := make([]DaemonResponse, 2)
	for i := range responses {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			payload, err := json.Marshal(DaemonRequest{
				Session: "same-session",
				Command: "launch",
				Args:    []string{"sleep", "5"},
			})
			if err != nil {
				t.Errorf("marshal request: %v", err)
				return
			}
			req := httptest.NewRequest("POST", "/cmd", bytes.NewReader(payload))
			response := httptest.NewRecorder()
			d.handleCommand(response, req)
			if err := json.Unmarshal(response.Body.Bytes(), &responses[i]); err != nil {
				t.Errorf("decode response: %v", err)
			}
		}(i)
	}
	wg.Wait()

	successes := 0
	for _, response := range responses {
		if response.Error == "" {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one launch to succeed, got %+v", responses)
	}
}
