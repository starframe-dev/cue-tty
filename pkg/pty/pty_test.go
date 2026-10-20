package pty

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStartWithExtraFilesInheritsDescriptor(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	backend, err := StartWithExtraFiles("sh", []string{"-c", "cat <&3"}, os.Environ(), "", 80, 24, []*os.File{reader})
	_ = reader.Close()
	if err != nil {
		_ = writer.Close()
		t.Fatalf("StartWithExtraFiles failed: %v", err)
	}
	defer backend.Close()

	if _, err := io.WriteString(writer, "listener-fd-ok\n"); err != nil {
		_ = writer.Close()
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	readDone := make(chan []byte, 1)
	go func() {
		output, _ := io.ReadAll(backend)
		readDone <- output
	}()
	var output []byte
	select {
	case output = <-readDone:
	case <-time.After(5 * time.Second):
		_ = backend.Close()
		t.Fatal("child did not finish reading inherited descriptor")
	}
	if err := backend.Wait(); err != nil {
		t.Fatalf("child process failed: %v; output=%q", err, output)
	}
	if !strings.Contains(string(output), "listener-fd-ok") {
		t.Fatalf("child did not read inherited descriptor: %q", output)
	}
}

func TestValidateSize(t *testing.T) {
	tests := []struct {
		name    string
		cols    int
		rows    int
		wantErr bool
	}{
		{name: "normal", cols: 80, rows: 24},
		{name: "large supported", cols: 1024, rows: 128},
		{name: "maximum Portalis area", cols: 1024, rows: 256},
		{name: "just over Portalis area", cols: 1024, rows: 257, wantErr: true},
		{name: "area too large", cols: 1024, rows: 1024, wantErr: true},
		{name: "zero cols", cols: 0, rows: 24, wantErr: true},
		{name: "negative rows", cols: 80, rows: -1, wantErr: true},
		{name: "uint16 overflow", cols: 65536, rows: 24, wantErr: true},
		{name: "dimension too large", cols: 4097, rows: 1, wantErr: true},
		{name: "division overflow prevented", cols: int(^uint(0) >> 1), rows: 2, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSize(tt.cols, tt.rows)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateSize(%d, %d) error = %v, wantErr=%v", tt.cols, tt.rows, err, tt.wantErr)
			}
		})
	}
}
