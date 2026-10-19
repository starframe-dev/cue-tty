package pty

import "testing"

func TestValidateSize(t *testing.T) {
	tests := []struct {
		name    string
		cols    int
		rows    int
		wantErr bool
	}{
		{name: "normal", cols: 80, rows: 24},
		{name: "maximum area", cols: 1024, rows: 1024},
		{name: "zero cols", cols: 0, rows: 24, wantErr: true},
		{name: "negative rows", cols: 80, rows: -1, wantErr: true},
		{name: "uint16 overflow", cols: 65536, rows: 1, wantErr: true},
		{name: "dimension too large", cols: 4097, rows: 1, wantErr: true},
		{name: "area too large", cols: 2048, rows: 1024, wantErr: true},
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
