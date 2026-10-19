package ansi

import (
	"github.com/starframe-dev/portalis"
	"unicode/utf8"
)

// Parser feeds raw terminal bytes to Portalis' terminal parser and screen.
// Portalis owns ANSI parsing so terminal behavior is not duplicated here.
type Parser struct {
	parser *portalis.Parser
}

// NewParser creates a parser bound to a screen.
func NewParser(screen *Screen) *Parser {
	return &Parser{parser: portalis.NewParser(screen.Screen)}
}

// Feed processes a chunk of terminal output. Portalis preserves parser state
// across calls, including incomplete escape sequences split between reads.
func (p *Parser) Feed(data []byte) {
	p.parser.Feed(data)
}

// FeedString processes a string of raw terminal bytes.
func (p *Parser) FeedString(s string) {
	p.Feed([]byte(s))
}

// SetResponseCallback registers a callback for terminal-generated responses
// such as cursor position reports. The callback runs synchronously from Feed.
func (p *Parser) SetResponseCallback(callback func([]byte)) {
	p.parser.SetResponseCallback(callback)
}

// FeedRune processes a single rune.
func (p *Parser) FeedRune(r rune) {
	if r < utf8.RuneSelf {
		p.Feed([]byte{byte(r)})
		return
	}
	p.Feed([]byte(string(r)))
}
