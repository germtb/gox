package lexer

import (
	"unicode"
	"unicode/utf8"
)

// Lexer tokenizes a gox source file containing Go + JSX.
type Lexer struct {
	input  string
	pos    int // current position in input
	line   int // current line number (1-indexed)
	column int // current column number (1-indexed)

	// Mode tracking
	inJSX        bool // are we inside a JSX element?
	jsxDepth     int  // nesting depth of JSX elements
	braceDepth   int  // brace depth for expressions
	inTag        bool // are we inside an opening tag (before >)?
	inClosingTag bool // are we inside a closing tag (</tag>)?
	sawSlash     bool // did we just see a / (for self-closing)?
	needTagName  bool // is the next identifier a tag name?
}

// New creates a new Lexer for the given input.
func New(input string) *Lexer {
	return &Lexer{
		input:  input,
		pos:    0,
		line:   1,
		column: 1,
	}
}

// NextToken returns the next token from the input.
func (l *Lexer) NextToken() Token {
	if l.pos >= len(l.input) {
		return l.makeToken(TOKEN_EOF, "")
	}

	if l.inJSX {
		return l.lexJSX()
	}

	return l.lexGoCode()
}

// lexGoCode lexes Go code until we find a JSX element.
func (l *Lexer) lexGoCode() Token {
	start := l.pos
	startLine := l.line
	startColumn := l.column

	for l.pos < len(l.input) {
		if l.isJSXStart() {
			// Found JSX, return accumulated Go code first
			if l.pos > start {
				return Token{
					Type:   TOKEN_GO_CODE,
					Value:  l.input[start:l.pos],
					Offset: start,
					Line:   startLine,
					Column: startColumn,
				}
			}
			l.inJSX = true
			return l.lexJSXOpen()
		}

		// Handle strings and comments to avoid false JSX detection
		ch := l.peek()
		if ch == '"' {
			l.lexGoString()
		} else if ch == '\'' {
			l.lexGoRune()
		} else if ch == '`' {
			l.lexGoRawString()
		} else if ch == '/' && l.peekNext() == '/' {
			l.lexGoLineComment()
		} else if ch == '/' && l.peekNext() == '*' {
			l.lexGoBlockComment()
		} else {
			l.advance()
		}
	}

	// Return remaining Go code
	if l.pos > start {
		return Token{
			Type:   TOKEN_GO_CODE,
			Value:  l.input[start:l.pos],
			Offset: start,
			Line:   startLine,
			Column: startColumn,
		}
	}

	return l.makeToken(TOKEN_EOF, "")
}

// isJSXStart checks if we're at the start of a JSX element.
// JSX starts with < followed by an identifier or > (fragment).
func (l *Lexer) isJSXStart() bool {
	if l.peek() != '<' {
		return false
	}

	// Look ahead past the <
	nextPos := l.pos + 1
	if nextPos >= len(l.input) {
		return false
	}

	next := rune(l.input[nextPos])

	// Fragment: <>
	if next == '>' {
		return true
	}

	// Closing fragment: </>
	if next == '/' {
		if nextPos+1 < len(l.input) && l.input[nextPos+1] == '>' {
			return true
		}
	}

	// Element: <identifier or <Identifier
	if isIdentStart(next) {
		return true
	}

	return false
}

// lexJSX handles lexing within JSX context.
func (l *Lexer) lexJSX() Token {
	l.skipWhitespaceInTag()

	if l.pos >= len(l.input) {
		return l.makeToken(TOKEN_EOF, "")
	}

	ch := l.peek()

	// Handle expression braces
	if ch == '{' {
		return l.lexJSXExpression()
	}

	if ch == '}' && l.braceDepth > 0 {
		l.advance()
		l.braceDepth--
		return l.makeToken(TOKEN_JSX_RBRACE, "}")
	}

	// Handle tags
	if ch == '<' {
		return l.lexJSXOpen()
	}

	if ch == '>' {
		l.advance()
		wasClosingTag := l.inClosingTag
		wasSelfClosing := l.sawSlash
		l.inTag = false
		l.inClosingTag = false
		l.sawSlash = false
		if wasClosingTag || wasSelfClosing {
			l.jsxDepth--
			if l.jsxDepth == 0 {
				l.inJSX = false
			}
		}
		return l.makeToken(TOKEN_JSX_CLOSE, ">")
	}

	if ch == '/' {
		l.advance()
		l.sawSlash = true
		return l.makeToken(TOKEN_JSX_SLASH, "/")
	}

	// Inside tag: attributes
	if l.inTag {
		if ch == '=' {
			l.advance()
			return l.makeToken(TOKEN_JSX_EQUALS, "=")
		}

		if ch == '"' {
			return l.lexJSXString()
		}

		if ch == '.' && l.peekNext() == '.' && l.peekAt(2) == '.' {
			l.advance()
			l.advance()
			l.advance()
			return l.makeToken(TOKEN_JSX_SPREAD, "...")
		}

		if isIdentStart(ch) {
			if l.needTagName {
				l.needTagName = false
				return l.lexJSXIdentifier(TOKEN_JSX_TAG)
			}
			return l.lexJSXIdentifier(TOKEN_JSX_ATTR_NAME)
		}
	}

	// Text content between tags
	return l.lexJSXText()
}

// lexJSXOpen handles < at the start of a JSX element or fragment.
func (l *Lexer) lexJSXOpen() Token {
	start := l.pos
	l.advance() // consume <

	if l.peek() == '>' {
		// Fragment open: <>
		l.advance()
		l.inTag = false
		l.jsxDepth++
		return Token{
			Type:   TOKEN_JSX_FRAG_OPEN,
			Value:  "<>",
			Offset: start,
			Line:   l.line,
			Column: l.column - 2,
		}
	}

	if l.peek() == '/' {
		l.advance()
		if l.peek() == '>' {
			// Fragment close: </>
			l.advance()
			l.jsxDepth--
			if l.jsxDepth == 0 {
				l.inJSX = false
			}
			return Token{
				Type:   TOKEN_JSX_FRAG_CLOSE,
				Value:  "</>",
				Offset: start,
				Line:   l.line,
				Column: l.column - 3,
			}
		}
		// Closing tag: </tag>
		l.inTag = true
		l.inClosingTag = true
		l.needTagName = true
		return Token{
			Type:   TOKEN_JSX_OPEN,
			Value:  "</",
			Offset: start,
			Line:   l.line,
			Column: l.column - 2,
		}
	}

	// Opening tag: <tag
	l.inTag = true
	l.needTagName = true
	l.jsxDepth++
	return Token{
		Type:   TOKEN_JSX_OPEN,
		Value:  "<",
		Offset: start,
		Line:   l.line,
		Column: l.column - 1,
	}
}

// lexJSXIdentifier lexes a JSX identifier (tag name or attribute name).
func (l *Lexer) lexJSXIdentifier(typ TokenType) Token {
	start := l.pos
	startLine := l.line
	startColumn := l.column

	for l.pos < len(l.input) {
		ch := l.peek()
		if !isIdentChar(ch) && ch != '-' { // Allow hyphens in JSX identifiers
			break
		}
		l.advance()
	}

	return Token{
		Type:   typ,
		Value:  l.input[start:l.pos],
		Offset: start,
		Line:   startLine,
		Column: startColumn,
	}
}

// lexJSXString lexes a double-quoted string attribute value.
func (l *Lexer) lexJSXString() Token {
	start := l.pos
	startLine := l.line
	startColumn := l.column

	l.advance() // consume opening "

	for l.pos < len(l.input) && l.peek() != '"' {
		if l.peek() == '\\' && l.peekNext() == '"' {
			l.advance() // skip backslash
		}
		l.advance()
	}

	l.advance() // consume closing "

	// Return value without quotes
	return Token{
		Type:   TOKEN_JSX_STRING,
		Value:  l.input[start+1 : l.pos-1],
		Offset: start,
		Line:   startLine,
		Column: startColumn,
	}
}

// lexJSXExpression lexes a JSX expression {expr}.
func (l *Lexer) lexJSXExpression() Token {
	start := l.pos
	startLine := l.line
	startColumn := l.column

	l.advance() // consume {
	l.braceDepth = 1

	exprStart := l.pos
	for l.pos < len(l.input) && l.braceDepth > 0 {
		ch := l.peek()

		if ch == '{' {
			l.braceDepth++
		} else if ch == '}' {
			l.braceDepth--
			if l.braceDepth == 0 {
				break
			}
		} else if ch == '"' {
			l.lexGoString()
			continue
		} else if ch == '\'' {
			l.lexGoRune()
			continue
		} else if ch == '`' {
			l.lexGoRawString()
			continue
		} else if ch == '<' && l.isJSXStart() {
			// Nested JSX in expression - include it in the expression
			l.lexNestedJSX()
			continue
		}

		l.advance()
	}

	expr := l.input[exprStart:l.pos]
	l.advance() // consume closing }
	l.braceDepth = 0

	return Token{
		Type:   TOKEN_JSX_EXPR,
		Value:  expr,
		Offset: start,
		Line:   startLine,
		Column: startColumn,
	}
}

// lexNestedJSX consumes a nested JSX element within an expression.
func (l *Lexer) lexNestedJSX() {
	depth := 0

	for l.pos < len(l.input) {
		ch := l.peek()

		if ch == '<' {
			if l.peekNext() == '/' {
				// Could be closing tag or fragment
				l.advance()
				l.advance()
				if l.peek() == '>' {
					// Fragment close
					l.advance()
					depth--
				} else {
					// Closing tag
					for l.pos < len(l.input) && l.peek() != '>' {
						l.advance()
					}
					if l.peek() == '>' {
						l.advance()
					}
					depth--
				}
			} else if l.peekNext() == '>' {
				// Fragment open
				l.advance()
				l.advance()
				depth++
			} else if isIdentStart(l.peekNext()) {
				// Opening tag
				l.advance()
				depth++
				// Find the end of the tag
				for l.pos < len(l.input) {
					if l.peek() == '>' {
						l.advance()
						break
					}
					if l.peek() == '/' && l.peekNext() == '>' {
						l.advance()
						l.advance()
						depth--
						break
					}
					l.advance()
				}
			} else {
				l.advance()
			}
		} else {
			l.advance()
		}

		if depth == 0 {
			break
		}
	}
}

// lexJSXText lexes text content between JSX tags.
func (l *Lexer) lexJSXText() Token {
	start := l.pos
	startLine := l.line
	startColumn := l.column

	for l.pos < len(l.input) {
		ch := l.peek()
		if ch == '<' || ch == '{' || ch == '}' {
			break
		}
		l.advance()
	}

	text := l.input[start:l.pos]
	return Token{
		Type:   TOKEN_JSX_TEXT,
		Value:  text,
		Offset: start,
		Line:   startLine,
		Column: startColumn,
	}
}

// Helper functions

func (l *Lexer) peek() rune {
	if l.pos >= len(l.input) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.input[l.pos:])
	return r
}

func (l *Lexer) peekNext() rune {
	if l.pos+1 >= len(l.input) {
		return 0
	}
	_, size := utf8.DecodeRuneInString(l.input[l.pos:])
	r, _ := utf8.DecodeRuneInString(l.input[l.pos+size:])
	return r
}

func (l *Lexer) peekAt(offset int) rune {
	pos := l.pos + offset
	if pos >= len(l.input) {
		return 0
	}
	r, _ := utf8.DecodeRuneInString(l.input[pos:])
	return r
}

func (l *Lexer) advance() {
	if l.pos >= len(l.input) {
		return
	}

	r, size := utf8.DecodeRuneInString(l.input[l.pos:])
	if r == '\n' {
		l.line++
		l.column = 1
	} else {
		l.column++
	}
	l.pos += size
}

func (l *Lexer) makeToken(typ TokenType, value string) Token {
	return Token{
		Type:   typ,
		Value:  value,
		Offset: l.pos - len(value),
		Line:   l.line,
		Column: l.column - len(value),
	}
}

func (l *Lexer) skipWhitespaceInTag() {
	if !l.inTag {
		return
	}
	for l.pos < len(l.input) {
		ch := l.peek()
		if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' {
			break
		}
		l.advance()
	}
}

// lexGoString skips over a Go string literal.
func (l *Lexer) lexGoString() {
	l.advance() // consume opening "
	for l.pos < len(l.input) {
		ch := l.peek()
		if ch == '"' {
			l.advance()
			break
		}
		if ch == '\\' {
			l.advance()
		}
		l.advance()
	}
}

// lexGoRune skips over a Go rune literal.
func (l *Lexer) lexGoRune() {
	l.advance() // consume opening '
	for l.pos < len(l.input) {
		ch := l.peek()
		if ch == '\'' {
			l.advance()
			break
		}
		if ch == '\\' {
			l.advance()
		}
		l.advance()
	}
}

// lexGoRawString skips over a Go raw string literal.
func (l *Lexer) lexGoRawString() {
	l.advance() // consume opening `
	for l.pos < len(l.input) {
		if l.peek() == '`' {
			l.advance()
			break
		}
		l.advance()
	}
}

// lexGoLineComment skips over a Go line comment.
func (l *Lexer) lexGoLineComment() {
	for l.pos < len(l.input) && l.peek() != '\n' {
		l.advance()
	}
}

// lexGoBlockComment skips over a Go block comment.
func (l *Lexer) lexGoBlockComment() {
	l.advance() // consume /
	l.advance() // consume *
	for l.pos < len(l.input) {
		if l.peek() == '*' && l.peekNext() == '/' {
			l.advance()
			l.advance()
			break
		}
		l.advance()
	}
}

func isIdentStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isIdentChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
