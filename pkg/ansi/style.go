package ansi

// Color represents a terminal color.
type Color struct {
	R, G, B uint8
	Is256   bool
	Index   uint8
}

// Default colors for indices 0-15.
var DefaultPalette = [256]Color{
	{0, 0, 0, true, 0},         // black
	{205, 49, 49, true, 1},     // red
	{13, 188, 121, true, 2},    // green
	{229, 229, 16, true, 3},    // yellow
	{36, 114, 200, true, 4},    // blue
	{188, 63, 188, true, 5},    // magenta
	{17, 168, 205, true, 6},    // cyan
	{229, 229, 229, true, 7},   // white
	{102, 102, 102, true, 8},   // bright black
	{241, 76, 76, true, 9},     // bright red
	{35, 209, 139, true, 10},   // bright green
	{245, 245, 67, true, 11},   // bright yellow
	{59, 142, 234, true, 12},   // bright blue
	{214, 112, 214, true, 13},  // bright magenta
	{41, 184, 219, true, 14},   // bright cyan
	{255, 255, 255, true, 15},  // bright white
}

func init() {
	// Indices 16-231: 6x6x6 color cube
	for r := 0; r < 6; r++ {
		for g := 0; g < 6; g++ {
			for b := 0; b < 6; b++ {
				i := 16 + r*36 + g*6 + b
				if i < 256 {
					DefaultPalette[i] = Color{
						R: uint8(r * 51), G: uint8(g * 51), B: uint8(b * 51),
						Is256: true, Index: uint8(i),
					}
				}
			}
		}
	}
	// Indices 232-255: grayscale ramp
	for i := 0; i < 24; i++ {
		v := uint8(8 + i*10)
		DefaultPalette[232+i] = Color{R: v, G: v, B: v, Is256: true, Index: uint8(232 + i)}
	}
}

// Style describes the visual appearance of a cell.
type Style struct {
	Fg        Color
	Bg        Color
	Bold      bool
	Dim       bool
	Italic    bool
	Underline bool
	Reverse   bool
}

// Reset returns a zero style.
func Reset() Style {
	return Style{Fg: Color{229, 229, 229, true, 7}, Bg: Color{0, 0, 0, true, 0}}
}
