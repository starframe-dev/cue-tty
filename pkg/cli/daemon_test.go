package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/starframe-dev/cue-tty/pkg/cue"
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

func TestConcurrentShowListLaunchClose(t *testing.T) {
	d := NewDaemon()
	defer d.sm.CloseAll()
	if response := callDaemonCommand(t, d, DaemonRequest{Session: "race-session", Command: "launch", Args: []string{"sleep", "10"}}); response.Error != "" {
		t.Fatal(response.Error)
	}

	var wg sync.WaitGroup
	errs := make(chan string, 120)
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				command := "show"
				if (worker+i)%2 == 0 {
					command = "list"
				}
				response := callDaemonCommand(t, d, DaemonRequest{Command: command})
				if response.Error != "" {
					errs <- response.Error
				}
			}
		}(worker)
	}
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_ = callDaemonCommand(t, d, DaemonRequest{Session: "race-session", Command: "launch", Args: []string{"sleep", "1"}})
			_ = callDaemonCommand(t, d, DaemonRequest{Session: "race-session", Command: "close"})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_ = d.sm.Close("race-session")
		}
	}()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("concurrent show/list/launch/close did not finish")
	}
	close(errs)
	for err := range errs {
		t.Errorf("dashboard command failed: %s", err)
	}
}

func TestDaemonCloseCancelsBlockedPTYInput(t *testing.T) {
	d := NewDaemon()
	defer d.sm.CloseAll()
	if response := callDaemonCommand(t, d, DaemonRequest{Session: "blocked", Command: "launch", Args: []string{"sh", "-c", "exec sleep 30"}}); response.Error != "" {
		t.Fatal(response.Error)
	}
	session, ok := d.sm.Get("blocked")
	if !ok {
		t.Fatal("session was not created")
	}

	typeResult := make(chan DaemonResponse, 1)
	go func() {
		typeResult <- callDaemonCommand(t, d, DaemonRequest{
			Session: "blocked", Command: "type", Args: []string{strings.Repeat("x", 512<<10)},
		})
	}()
	waitForActiveCommand(t, session)

	started := time.Now()
	response := callDaemonCommand(t, d, DaemonRequest{Session: "blocked", Command: "close"})
	if response.Error != "" {
		t.Fatalf("close failed: %s", response.Error)
	}
	select {
	case result := <-typeResult:
		if result.Error == "" {
			t.Fatal("blocked type command unexpectedly succeeded")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("blocked type command did not cancel after close")
	}
	if time.Since(started) > 3*time.Second {
		t.Fatalf("close took too long: %s", time.Since(started))
	}
	if session.App != nil {
		t.Fatal("close left the app attached to the session")
	}
}

func waitForActiveCommand(t *testing.T, session *Session) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		session.cancelMu.Lock()
		active := session.activeCancel != nil
		session.cancelMu.Unlock()
		if active {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("command did not become active")
}

func TestDaemonHTTPWaitTimeoutsAndLifecycle(t *testing.T) {
	d := NewDaemon()
	dir, err := os.MkdirTemp("/tmp", "ctd-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	d.socketPath = filepath.Join(dir, "d.sock")
	runErr := make(chan error, 1)
	go func() { runErr <- d.Run() }()
	defer func() { _ = d.Stop() }()

	deadline := time.Now().Add(3 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", d.socketPath, 20*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			ready = true
			break
		}
		select {
		case runErr := <-runErr:
			t.Fatalf("daemon exited before readiness: %v", runErr)
		default:
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		t.Fatal("daemon Unix socket did not become ready")
	}
	client := &http.Client{
		Timeout: clientTimeout,
		Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", d.socketPath)
		}},
	}
	send := func(request DaemonRequest) (DaemonResponse, error) {
		payload, err := json.Marshal(request)
		if err != nil {
			return DaemonResponse{}, err
		}
		response, err := client.Post("http://daemon/cmd", "application/json", bytes.NewReader(payload))
		if err != nil {
			return DaemonResponse{}, err
		}
		defer response.Body.Close()
		var result DaemonResponse
		err = json.NewDecoder(response.Body).Decode(&result)
		return result, err
	}

	response, err := send(DaemonRequest{Session: "lifecycle", Command: "launch", Args: []string{"sh", "-c", "printf ready; exec sleep 30"}})
	if err != nil || response.Error != "" {
		t.Fatalf("launch: response=%+v err=%v", response, err)
	}
	response, err = send(DaemonRequest{Session: "lifecycle", Command: "waitfor", Args: []string{"ready", "60000"}})
	if err != nil || response.Error != "" {
		t.Fatalf("long-timeout waitfor: response=%+v err=%v", response, err)
	}

	waitResult := make(chan DaemonResponse, 1)
	go func() {
		result, _ := send(DaemonRequest{Session: "lifecycle", Command: "wait", Args: []string{"10000"}})
		waitResult <- result
	}()
	session, _ := d.sm.Get("lifecycle")
	waitForActiveCommand(t, session)
	response, err = send(DaemonRequest{Session: "lifecycle", Command: "close"})
	if err != nil || response.Error != "" {
		t.Fatalf("close while wait active: response=%+v err=%v", response, err)
	}
	select {
	case result := <-waitResult:
		if result.Error == "" {
			t.Fatal("wait unexpectedly completed instead of being canceled")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("wait command did not cancel after close")
	}

	response, err = send(DaemonRequest{Session: "lifecycle", Command: "launch", Args: []string{"sleep", "30"}})
	if err != nil || response.Error != "" {
		t.Fatalf("second launch: response=%+v err=%v", response, err)
	}
	session, _ = d.sm.Get("lifecycle")
	exitedApp := session.App
	response, err = send(DaemonRequest{Session: "lifecycle", Command: "exit"})
	if err != nil || response.Error != "" {
		t.Fatalf("exit: response=%+v err=%v", response, err)
	}
	if _, ok := d.sm.Get("lifecycle"); ok {
		t.Fatal("exit left the session in SessionManager")
	}
	if exitedApp.ProcessState() == nil {
		t.Fatal("exit left its process unreaped")
	}
	response, err = send(DaemonRequest{Session: "shutdown", Command: "launch", Args: []string{"sleep", "30"}})
	if err != nil || response.Error != "" {
		t.Fatalf("shutdown-test launch: response=%+v err=%v", response, err)
	}
	shutdownSession, _ := d.sm.Get("shutdown")
	shutdownApp := shutdownSession.App

	response, err = send(DaemonRequest{Command: "stop"})
	if err != nil || response.Error != "" {
		t.Fatalf("stop: response=%+v err=%v", response, err)
	}
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("daemon Run: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("daemon did not stop")
	}
	if _, err := os.Stat(d.socketPath); !os.IsNotExist(err) {
		t.Fatalf("daemon socket was not cleaned up: %v", err)
	}
	if shutdownApp.ProcessState() == nil {
		t.Fatal("daemon shutdown left its child process unreaped")
	}
	if len(d.sm.List()) != 0 {
		t.Fatalf("daemon shutdown left sessions: %v", d.sm.List())
	}
}

func TestDaemonRequestCancellationInterruptsWait(t *testing.T) {
	d := NewDaemon()
	defer d.sm.CloseAll()
	if response := callDaemonCommand(t, d, DaemonRequest{Session: "cancel-request", Command: "launch", Args: []string{"sleep", "10"}}); response.Error != "" {
		t.Fatal(response.Error)
	}
	session, _ := d.sm.Get("cancel-request")
	ctx, cancel := context.WithCancel(context.Background())
	payload, err := json.Marshal(DaemonRequest{Session: "cancel-request", Command: "wait", Args: []string{"10000"}})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest("POST", "/cmd", bytes.NewReader(payload)).WithContext(ctx)
	response := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		d.handleCommand(response, request)
		close(done)
	}()
	waitForActiveCommand(t, session)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("request cancellation did not interrupt wait")
	}
	var result DaemonResponse
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Error, context.Canceled.Error()) {
		t.Fatalf("cancelled wait response = %+v", result)
	}
}

func TestWaitDurationValidationAndCancellation(t *testing.T) {
	app, err := cue.Launch("sh", cue.WithArgs("-c", "printf ready; exec sleep 5"))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	session := &Session{App: app, Page: app.Page()}
	if err := app.Page().WaitFor("ready", time.Second); err != nil {
		t.Fatal(err)
	}
	for _, ms := range []string{"-1", "60001", "99999999999999999999", "abc"} {
		if _, err := cmdWait(session, []string{ms}); err == nil {
			t.Errorf("wait %s unexpectedly succeeded", ms)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	session.commandContext = ctx
	cancel()
	if _, err := cmdWait(session, []string{"10000"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait 10000 error = %v, want context canceled", err)
	}
	if _, err := cmdWaitFor(session, []string{"ready", "9223372036854775808"}); err == nil {
		t.Fatal("waitfor accepted an overflowing timeout")
	}
	if _, err := cmdWaitFor(session, []string{"ready", "-1"}); err == nil {
		t.Fatal("waitfor accepted a negative timeout")
	}
	if _, err := cmdWaitFor(session, []string{"ready", "60000"}); err != nil {
		t.Fatalf("waitfor rejected the maximum supported timeout: %v", err)
	}
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
