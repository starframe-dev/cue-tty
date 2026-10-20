package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseCLIArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantSession string
		wantCommand []string
		wantErr     bool
	}{
		{
			name:        "short flag only before command",
			args:        []string{"-s", "demo", "launch", "vim", "-s", "script.vim"},
			wantSession: "demo",
			wantCommand: []string{"launch", "vim", "-s", "script.vim"},
		},
		{
			name:        "long flag only before command",
			args:        []string{"--session", "demo", "launch", "app", "--session", "child-value"},
			wantSession: "demo",
			wantCommand: []string{"launch", "app", "--session", "child-value"},
		},
		{
			name:        "short equal form",
			args:        []string{"-s=demo", "list"},
			wantSession: "demo",
			wantCommand: []string{"list"},
		},
		{
			name:        "long equal form",
			args:        []string{"--session=demo", "show"},
			wantSession: "demo",
			wantCommand: []string{"show"},
		},
		{
			name:        "explicit delimiter",
			args:        []string{"-s", "demo", "--", "launch", "vim", "-s", "script.vim"},
			wantSession: "demo",
			wantCommand: []string{"launch", "vim", "-s", "script.vim"},
		},
		{
			name:        "delimiter makes leading flag the command",
			args:        []string{"--", "-s", "value"},
			wantSession: "default",
			wantCommand: []string{"-s", "value"},
		},
		{
			name:        "exact child arguments",
			args:        []string{"-s", "demo", "launch", "app", "", "with spaces", "Привет世界", `quote\"`, "$HOME;*"},
			wantSession: "demo",
			wantCommand: []string{"launch", "app", "", "with spaces", "Привет世界", `quote\"`, "$HOME;*"},
		},
		{
			name:        "flags without command start interactive mode",
			args:        []string{"-s", "demo"},
			wantSession: "demo",
		},
		{
			name:    "missing session value",
			args:    []string{"--session"},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotSession, gotCommand, err := parseCLIArgs(test.args)
			if (err != nil) != test.wantErr {
				t.Fatalf("parseCLIArgs error = %v, wantErr %v", err, test.wantErr)
			}
			if err != nil {
				return
			}
			if gotSession != test.wantSession {
				t.Errorf("session = %q, want %q", gotSession, test.wantSession)
			}
			if !reflect.DeepEqual(gotCommand, test.wantCommand) {
				t.Errorf("command = %#v, want %#v", gotCommand, test.wantCommand)
			}
		})
	}
}

func TestRunPassesExactArgumentsToChild(t *testing.T) {
	capturePath := filepath.Join(t.TempDir(), "args.json")
	want := []string{"-s", "script.vim", "with spaces", "Привет世界", "", `quoted "text"`, "$HOME;*`"}
	args := []string{
		"-s", "argv-session", "launch", os.Args[0],
		"-test.run=^TestCLIArgumentCaptureHelper$", "--", capturePath,
	}
	args = append(args, want...)

	var gotSession, gotCommand string
	var gotCommandArgs []string
	err := runWithSender(args, func(session, command string, commandArgs []string) (string, error) {
		gotSession = session
		gotCommand = command
		gotCommandArgs = append([]string(nil), commandArgs...)
		cmd := exec.Command(commandArgs[0], commandArgs[1:]...)
		cmd.Env = append(os.Environ(), "CUE_TTY_CAPTURE_ARGS=1")
		return "", cmd.Run()
	})
	if err != nil {
		t.Fatalf("runWithSender: %v", err)
	}
	if gotSession != "argv-session" || gotCommand != "launch" {
		t.Fatalf("received session=%q command=%q", gotSession, gotCommand)
	}
	if !reflect.DeepEqual(gotCommandArgs, args[3:]) {
		t.Fatalf("launch args = %#v, want %#v", gotCommandArgs, args[3:])
	}

	data, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("decode child args: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("child received %#v, want %#v", got, want)
	}
}

func TestCLIArgumentCaptureHelper(t *testing.T) {
	if os.Getenv("CUE_TTY_CAPTURE_ARGS") != "1" {
		return
	}
	separator := -1
	for i, arg := range os.Args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator < 0 || separator+1 >= len(os.Args) {
		t.Fatal("missing argument capture path")
	}
	data, err := json.Marshal(os.Args[separator+2:])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Args[separator+1], data, 0600); err != nil {
		t.Fatal(err)
	}
}
