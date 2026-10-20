package cli

import (
	"strconv"
	"testing"

	"github.com/starframe-dev/cue-tty/pkg/cue"
)

func TestCmdResizeMatchesPTYAndPortalisLimits(t *testing.T) {
	app, err := cue.Launch("sleep", cue.WithArgs("30"), cue.WithSize(80, 24))
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	session := &Session{Name: "resize", App: app, Page: app.Page()}

	tests := []struct {
		cols    int
		rows    int
		wantErr bool
	}{
		{cols: 80, rows: 24},
		{cols: 1024, rows: 128},
		{cols: 1024, rows: 256},
		{cols: 1024, rows: 257, wantErr: true},
		{cols: 1024, rows: 1024, wantErr: true},
		{cols: 0, rows: 24, wantErr: true},
		{cols: 65536, rows: 24, wantErr: true},
	}
	for _, test := range tests {
		previousScreen := app.TerminalSnapshot()
		_, err = cmdResize(session, []string{strconv.Itoa(test.cols), strconv.Itoa(test.rows)})
		if (err != nil) != test.wantErr {
			t.Fatalf("resize %dx%d error = %v, wantErr %v", test.cols, test.rows, err, test.wantErr)
		}

		screen := app.TerminalSnapshot()
		if test.wantErr {
			if screen.Cols != previousScreen.Cols || screen.Rows != previousScreen.Rows {
				t.Fatalf("CLI resize error changed screen to %dx%d, prior size %dx%d", screen.Cols, screen.Rows, previousScreen.Cols, previousScreen.Rows)
			}
			continue
		}
		if screen.Cols != test.cols || screen.Rows != test.rows {
			t.Fatalf("CLI resize screen dimensions = %dx%d, want=%dx%d", screen.Cols, screen.Rows, test.cols, test.rows)
		}
	}
}
