package ansi

import (
	"strings"
	"testing"
)

func BenchmarkStreamOutput(b *testing.B) {
	stream := []byte(strings.Repeat("\x1b[32mwide界 text\x1b[0m\r\n", 24))
	for _, size := range []struct {
		name       string
		cols, rows int
	}{
		{name: "80x24", cols: 80, rows: 24},
		{name: "200x60", cols: 200, rows: 60},
	} {
		b.Run(size.name, func(b *testing.B) {
			screen := NewScreen(size.cols, size.rows)
			parser := NewParser(screen)
			b.SetBytes(int64(len(stream)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				parser.Feed(stream)
			}
		})
	}
}

func BenchmarkSnapshotRender(b *testing.B) {
	for _, size := range []struct {
		name       string
		cols, rows int
	}{
		{name: "80x24", cols: 80, rows: 24},
		{name: "200x60", cols: 200, rows: 60},
	} {
		b.Run(size.name, func(b *testing.B) {
			screen := NewScreen(size.cols, size.rows)
			parser := NewParser(screen)
			for row := 0; row < size.rows; row++ {
				parser.Feed([]byte(strings.Repeat("screen snapshot render ", size.cols/22+1) + "\r\n"))
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = screen.CellsView()
				_ = screen.RenderText()
			}
		})
	}
}
