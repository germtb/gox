package lexer

import "fmt"

// TokenType represents the type of a lexical token.
type TokenType int

const (
	TOKEN_EOF TokenType = iota
	TOKEN_ERROR

	// Go code (pass-through)
	TOKEN_GO_CODE

	// JSX tokens
	TOKEN_JSX_OPEN       // <
	TOKEN_JSX_CLOSE      // >
	TOKEN_JSX_SLASH      // /
	TOKEN_JSX_TAG        // element/component name
	TOKEN_JSX_ATTR_NAME  // attribute name
	TOKEN_JSX_EQUALS     // =
	TOKEN_JSX_STRING     // "value"
	TOKEN_JSX_LBRACE     // {
	TOKEN_JSX_RBRACE     // }
	TOKEN_JSX_TEXT       // text between tags
	TOKEN_JSX_EXPR       // expression content inside {}
	TOKEN_JSX_FRAG_OPEN  // <>
	TOKEN_JSX_FRAG_CLOSE // </>
)

// String returns a string representation of the token type.
func (t TokenType) String() string {
	switch t {
	case TOKEN_EOF:
		return "EOF"
	case TOKEN_ERROR:
		return "ERROR"
	case TOKEN_GO_CODE:
		return "GO_CODE"
	case TOKEN_JSX_OPEN:
		return "JSX_OPEN"
	case TOKEN_JSX_CLOSE:
		return "JSX_CLOSE"
	case TOKEN_JSX_SLASH:
		return "JSX_SLASH"
	case TOKEN_JSX_TAG:
		return "JSX_TAG"
	case TOKEN_JSX_ATTR_NAME:
		return "JSX_ATTR_NAME"
	case TOKEN_JSX_EQUALS:
		return "JSX_EQUALS"
	case TOKEN_JSX_STRING:
		return "JSX_STRING"
	case TOKEN_JSX_LBRACE:
		return "JSX_LBRACE"
	case TOKEN_JSX_RBRACE:
		return "JSX_RBRACE"
	case TOKEN_JSX_TEXT:
		return "JSX_TEXT"
	case TOKEN_JSX_EXPR:
		return "JSX_EXPR"
	case TOKEN_JSX_FRAG_OPEN:
		return "JSX_FRAG_OPEN"
	case TOKEN_JSX_FRAG_CLOSE:
		return "JSX_FRAG_CLOSE"
	default:
		return fmt.Sprintf("TOKEN(%d)", t)
	}
}

// Token represents a lexical token.
type Token struct {
	Type   TokenType
	Value  string
	Offset int
	Line   int
	Column int
}

// String returns a string representation of the token.
func (t Token) String() string {
	if len(t.Value) > 20 {
		return fmt.Sprintf("%s(%q...)", t.Type, t.Value[:20])
	}
	return fmt.Sprintf("%s(%q)", t.Type, t.Value)
}
