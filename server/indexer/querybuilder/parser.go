package querybuilder

import (
	"fmt"
	"strings"
	"unicode"
)

type TokenType int

const (
	TokenWord TokenType = iota
	TokenQuoted
	TokenAlternation
	TokenEOF
)

type Token struct {
	Type  TokenType
	Value string
	Parts []Token
}

type Lexer struct {
	input string
	pos   int
	char  rune
}

func New(input string) *Lexer {
	l := &Lexer{input: input}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.pos >= len(l.input) {
		l.char = 0
	} else {
		l.char = rune(l.input[l.pos])
	}
	l.pos++
}

func (l *Lexer) peekChar() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	return rune(l.input[l.pos])
}

func (l *Lexer) skipWhitespace() {
	for unicode.IsSpace(l.char) {
		l.readChar()
	}
}

func (l *Lexer) NextToken() (Token, error) {
	l.skipWhitespace()

	switch l.char {
	case 0:
		return Token{Type: TokenEOF}, nil
	case '"':
		return l.readQuoted()
	case '(':
		return l.readAlternation()
	default:
		return l.readWord()
	}
}

func (l *Lexer) readQuoted() (Token, error) {
	l.readChar()
	var builder strings.Builder

	for l.char != '"' && l.char != 0 {
		if l.char == '\\' && l.peekChar() == '"' {
			l.readChar()
			builder.WriteRune('"')
			l.readChar()
			continue
		}
		builder.WriteRune(l.char)
		l.readChar()
	}

	if l.char != '"' {
		return Token{}, fmt.Errorf("unclosed quoted string")
	}

	l.readChar()
	return Token{Type: TokenQuoted, Value: builder.String()}, nil
}

func (l *Lexer) readAlternation() (Token, error) {
	l.readChar()
	var builder strings.Builder
	depth := 1

	for depth > 0 && l.char != 0 {
		if l.char == '(' {
			depth++
		} else if l.char == ')' {
			depth--
			if depth == 0 {
				break
			}
		}
		builder.WriteRune(l.char)
		l.readChar()
	}

	if l.char != ')' {
		return Token{}, fmt.Errorf("unclosed alternation string")
	}

	l.readChar()
	value := builder.String()

	parts, err := parseAlternationParts(value)
	if err != nil {
		return Token{}, err
	}

	return Token{Type: TokenAlternation, Value: value, Parts: parts}, nil
}

func parseAlternationParts(value string) ([]Token, error) {
	parts := []Token{}
	var sb strings.Builder
	depth := 0

	for i, ch := range value {
		if ch == '(' {
			depth++
			sb.WriteRune(ch)
		} else if ch == ')' {
			depth--
			sb.WriteRune(ch)
		} else if ch == '|' && depth == 0 {
			optStr := strings.TrimSpace(sb.String())
			if optStr != "" {
				token := Token{Type: TokenWord, Value: optStr}
				parts = append(parts, token)
			}
			sb.Reset()
		} else {
			sb.WriteRune(ch)
		}

		if i == len(value)-1 {
			optStr := strings.TrimSpace(sb.String())
			if optStr != "" {
				token := Token{Type: TokenWord, Value: optStr}
				parts = append(parts, token)
			}
		}
	}

	if len(parts) == 0 {
		optStr := strings.TrimSpace(sb.String())
		if optStr != "" {
			token := Token{Type: TokenWord, Value: optStr}
			parts = append(parts, token)
		}
	}

	return parts, nil
}

func (l *Lexer) readWord() (Token, error) {
	var builder strings.Builder

	quote := false
	for quote || (!unicode.IsSpace(l.char) && l.char != 0) {
		if l.char == '"' {
			quote = !quote
		}
		builder.WriteRune(l.char)
		l.readChar()
	}

	return Token{Type: TokenWord, Value: builder.String()}, nil
}

func (t Token) String() {
	fmt.Printf("%d %s: %v\n", t.Type, t.Value, t.Parts)
}

func Tokenize(input string) ([]Token, error) {
	lexer := New(input)
	tokens := []Token{}

	for {
		token, err := lexer.NextToken()
		if err != nil {
			return nil, err
		}
		if token.Type == TokenEOF {
			break
		}
		tokens = append(tokens, token)
	}

	return tokens, nil
}
