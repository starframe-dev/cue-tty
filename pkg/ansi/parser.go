package ansi

import "unicode/utf8"

// Parser feeds raw terminal bytes into a Screen.
type Parser struct {
	screen   *Screen
	state    parseState
	params   []int
	intermed []byte
	pending  []byte
	csiPriv  bool // true if CSI started with ?
}

type parseState int

const (
	stateGround parseState = iota
	stateEscape
	stateCSI
	stateOSC
	stateOSCString
	stateSS2
	stateSS3
)

// NewParser creates a parser bound to a screen.
func NewParser(screen *Screen) *Parser {
	return &Parser{screen: screen}
}

// Feed processes a chunk of raw terminal bytes.
func (p *Parser) Feed(data []byte) {
	for _, b := range data {
		p.feedByte(b)
	}
}

// FeedString processes a string of raw terminal bytes.
func (p *Parser) FeedString(s string) {
	p.Feed([]byte(s))
}

func (p *Parser) feedByte(b byte) {
	switch p.state {
	case stateGround:
		p.handleGround(b)
	case stateEscape:
		p.handleEscape(b)
	case stateCSI:
		p.handleCSI(b)
	case stateOSC:
		p.handleOSC(b)
	case stateOSCString:
		p.handleOSCString(b)
	case stateSS2:
		p.handleSS2(b)
	case stateSS3:
		p.handleSS3(b)
	}
}

func (p *Parser) handleGround(b byte) {
	switch b {
	case 0x1B:
		p.state = stateEscape
	case 0x0D:
		p.screen.CurrentCursor().Col = 0
	case 0x0A, 0x0B, 0x0C:
		cur := p.screen.CurrentCursor()
		cur.Row++
		if cur.Row > p.screen.ScrollBottom {
			p.screen.scrollUp(1)
			cur.Row = p.screen.ScrollBottom
		}
	case 0x09:
		cur := p.screen.CurrentCursor()
		cur.Col = (cur.Col/8 + 1) * 8
		if cur.Col >= p.screen.Cols {
			cur.Col = p.screen.Cols - 1
		}
	case 0x08:
		cur := p.screen.CurrentCursor()
		if cur.Col > 0 {
			cur.Col--
		}
	case 0x07:
		// Bell — ignore
	default:
		if b >= 0x20 {
			p.screen.Put(rune(b))
		}
	}
}

func (p *Parser) handleEscape(b byte) {
	switch b {
	case '[':
		p.state = stateCSI
		p.params = p.params[:0]
		p.intermed = p.intermed[:0]
	case ']':
		p.state = stateOSC
		p.pending = p.pending[:0]
	case 'N':
		p.state = stateSS2
	case 'O':
		p.state = stateSS3
	case '(':
		p.state = stateSS2 // charset sequence, skip next
	case ')':
		p.state = stateSS2
	case '7':
		// Save cursor — not implemented yet
		p.state = stateGround
	case '8':
		// Restore cursor — not implemented yet
		p.state = stateGround
	default:
		p.state = stateGround
	}
}

func (p *Parser) handleCSI(b byte) {
	if b == '?' {
		p.csiPriv = true
	} else if b >= 0x30 && b <= 0x39 {
		// Digit — accumulate into last param
		if len(p.params) == 0 {
			p.params = append(p.params, 0)
		}
		p.params[len(p.params)-1] = p.params[len(p.params)-1]*10 + int(b-'0')
	} else if b == ';' {
		p.params = append(p.params, 0)
	} else if b >= 0x20 && b <= 0x2F {
		p.intermed = append(p.intermed, b)
	} else if b >= 0x40 && b <= 0x7E {
		p.executeCSI(b)
		p.state = stateGround
		p.params = p.params[:0]
		p.intermed = p.intermed[:0]
		p.csiPriv = false
	} else {
		// Invalid byte in CSI, abort
		p.state = stateGround
		p.params = p.params[:0]
		p.intermed = p.intermed[:0]
		p.csiPriv = false
	}
}

func (p *Parser) executeCSI(final byte) {
	param := func(idx int, defaultVal int) int {
		if idx < len(p.params) && p.params[idx] > 0 {
			return p.params[idx]
		}
		return defaultVal
	}

	switch final {
	case 'A': // CUU — cursor up
		p.screen.MoveCursorRel(-param(0, 1), 0)
	case 'B': // CUD — cursor down
		p.screen.MoveCursorRel(param(0, 1), 0)
	case 'C': // CUF — cursor forward
		p.screen.MoveCursorRel(0, param(0, 1))
	case 'D': // CUB — cursor back
		p.screen.MoveCursorRel(0, -param(0, 1))
	case 'E': // CNL — cursor next line
		cur := p.screen.CurrentCursor()
		p.screen.MoveCursor(cur.Row+param(0, 1), 0)
	case 'F': // CPL — cursor previous line
		cur := p.screen.CurrentCursor()
		p.screen.MoveCursor(cur.Row-param(0, 1), 0)
	case 'G': // CHA — cursor horizontal absolute
		cur := p.screen.CurrentCursor()
		p.screen.MoveCursor(cur.Row, param(0, 1)-1)
	case 'H', 'f': // CUP — cursor position
		p.screen.MoveCursor(param(0, 1)-1, param(1, 1)-1)
	case 'J': // ED — erase in display
		switch param(0, 0) {
		case 0:
			p.screen.ClearLine()
			cur := p.screen.CurrentCursor()
			cells := p.screen.Current()
			for r := cur.Row + 1; r < p.screen.Rows; r++ {
				for c := 0; c < p.screen.Cols; c++ {
					cells[r*p.screen.Cols+c] = EmptyCell()
				}
			}
		case 1:
			cur := p.screen.CurrentCursor()
			cells := p.screen.Current()
			for r := 0; r < cur.Row; r++ {
				for c := 0; c < p.screen.Cols; c++ {
					cells[r*p.screen.Cols+c] = EmptyCell()
				}
			}
			for c := 0; c <= cur.Col; c++ {
				cells[cur.Row*p.screen.Cols+c] = EmptyCell()
			}
		case 2:
			p.screen.ClearScreen()
		case 3:
			p.screen.ClearScreen()
		}
	case 'K': // EL — erase in line
		switch param(0, 0) {
		case 0:
			p.screen.ClearLine()
		case 1:
			p.screen.ClearLineLeft()
		case 2:
			p.screen.ClearLineAll()
		}
	case 'L': // IL — insert line
		// Not implemented
	case 'M': // DL — delete line
		// Not implemented
	case 'm': // SGR — select graphic rendition
		p.applySGR()
	case 'r': // DECSTBM — set scrolling region
		top := param(0, 1) - 1
		bottom := param(1, p.screen.Rows) - 1
		if top >= 0 && bottom < p.screen.Rows && top < bottom {
			p.screen.ScrollTop = top
			p.screen.ScrollBottom = bottom
		}
	case 's':
		// Save cursor — not implemented
	case 'u':
		// Restore cursor — not implemented
	case 'h': // SM / DECSET
		if p.csiPriv && len(p.params) > 0 && p.params[0] == 1049 {
			p.screen.SwitchAltBuffer(true)
		}
	case 'l': // RM / DECRST
		if p.csiPriv && len(p.params) > 0 && p.params[0] == 1049 {
			p.screen.SwitchAltBuffer(false)
		}
	}
}

func (p *Parser) applySGR() {
	if len(p.params) == 0 {
		p.params = append(p.params, 0)
	}
	for i := 0; i < len(p.params); i++ {
		code := p.params[i]
		switch {
		case code == 0:
			p.screen.Style = Reset()
		case code == 1:
			p.screen.Style.Bold = true
		case code == 2:
			p.screen.Style.Dim = true
		case code == 3:
			p.screen.Style.Italic = true
		case code == 4:
			p.screen.Style.Underline = true
		case code == 7:
			p.screen.Style.Reverse = true
		case code == 22:
			p.screen.Style.Bold = false
			p.screen.Style.Dim = false
		case code == 23:
			p.screen.Style.Italic = false
		case code == 24:
			p.screen.Style.Underline = false
		case code == 27:
			p.screen.Style.Reverse = false
		case code >= 30 && code <= 37:
			p.screen.Style.Fg = DefaultPalette[code-30]
		case code == 38:
			if i+1 < len(p.params) {
				sub := p.params[i+1]
				if sub == 5 && i+2 < len(p.params) {
					p.screen.Style.Fg = DefaultPalette[p.params[i+2]]
					i += 2
				} else if sub == 2 && i+4 < len(p.params) {
					p.screen.Style.Fg = Color{
						R: uint8(p.params[i+2]),
						G: uint8(p.params[i+3]),
						B: uint8(p.params[i+4]),
					}
					i += 4
				}
			}
		case code == 39:
			p.screen.Style.Fg = DefaultPalette[7]
		case code >= 40 && code <= 47:
			p.screen.Style.Bg = DefaultPalette[code-40]
		case code == 48:
			if i+1 < len(p.params) {
				sub := p.params[i+1]
				if sub == 5 && i+2 < len(p.params) {
					p.screen.Style.Bg = DefaultPalette[p.params[i+2]]
					i += 2
				} else if sub == 2 && i+4 < len(p.params) {
					p.screen.Style.Bg = Color{
						R: uint8(p.params[i+2]),
						G: uint8(p.params[i+3]),
						B: uint8(p.params[i+4]),
					}
					i += 4
				}
			}
		case code == 49:
			p.screen.Style.Bg = DefaultPalette[0]
		case code >= 90 && code <= 97:
			p.screen.Style.Fg = DefaultPalette[code-90+8]
		case code >= 100 && code <= 107:
			p.screen.Style.Bg = DefaultPalette[code-100+8]
		}
	}
}

func (p *Parser) handleOSC(b byte) {
	if b == 0x07 {
		p.state = stateGround
	} else if b == 0x1B {
		p.state = stateOSCString
	} else {
		p.pending = append(p.pending, b)
	}
}

func (p *Parser) handleOSCString(b byte) {
	if b == 0x5C {
		p.state = stateGround
	} else {
		p.pending = append(p.pending, b)
	}
}

func (p *Parser) handleSS2(b byte) {
	p.state = stateGround
}

func (p *Parser) handleSS3(b byte) {
	p.state = stateGround
}

// FeedRune processes a single rune (useful for tests).
func (p *Parser) FeedRune(r rune) {
	if r < utf8.RuneSelf {
		p.feedByte(byte(r))
	} else {
		p.Feed([]byte(string(r)))
	}
}
