package lexer

import (
	"testing"
)

func TestTokenTypeString(t *testing.T) {
	tests := []struct {
		typ      TokenType
		expected string
	}{
		{TOKEN_EOF, "EOF"},
		{TOKEN_GO_CODE, "GO_CODE"},
		{TOKEN_JSX_OPEN, "JSX_OPEN"},
		{TOKEN_JSX_TAG, "JSX_TAG"},
	}

	for _, tt := range tests {
		if got := tt.typ.String(); got != tt.expected {
			t.Errorf("TokenType.String() = %q, want %q", got, tt.expected)
		}
	}
}

func TestLexGoCode(t *testing.T) {
	input := `package main

func foo() int {
	return 42
}
`
	lex := New(input)
	tok := lex.NextToken()

	if tok.Type != TOKEN_GO_CODE {
		t.Errorf("Expected GO_CODE, got %v", tok.Type)
	}
	if tok.Value != input {
		t.Errorf("Expected full input, got %q", tok.Value)
	}

	tok = lex.NextToken()
	if tok.Type != TOKEN_EOF {
		t.Errorf("Expected EOF, got %v", tok.Type)
	}
}

func TestLexSimpleElement(t *testing.T) {
	input := `<box>`

	lex := New(input)

	tokens := collectTokens(lex)

	expected := []TokenType{
		TOKEN_JSX_OPEN,  // <
		TOKEN_JSX_TAG,   // box
		TOKEN_JSX_CLOSE, // >
		TOKEN_EOF,
	}

	assertTokenTypes(t, tokens, expected)
}

func TestLexSelfClosingElement(t *testing.T) {
	input := `<input />`

	lex := New(input)

	tokens := collectTokens(lex)

	expected := []TokenType{
		TOKEN_JSX_OPEN,  // <
		TOKEN_JSX_TAG,   // input
		TOKEN_JSX_SLASH, // /
		TOKEN_JSX_CLOSE, // >
		TOKEN_EOF,
	}

	assertTokenTypes(t, tokens, expected)
}

func TestLexElementWithStringAttribute(t *testing.T) {
	input := `<box direction="row">`

	lex := New(input)

	tokens := collectTokens(lex)

	expected := []TokenType{
		TOKEN_JSX_OPEN,      // <
		TOKEN_JSX_TAG,       // box
		TOKEN_JSX_ATTR_NAME, // direction
		TOKEN_JSX_EQUALS,    // =
		TOKEN_JSX_STRING,    // row
		TOKEN_JSX_CLOSE,     // >
		TOKEN_EOF,
	}

	assertTokenTypes(t, tokens, expected)

	// Check attribute value
	if tokens[4].Value != "row" {
		t.Errorf("Expected string value 'row', got %q", tokens[4].Value)
	}
}

func TestLexElementWithExpressionAttribute(t *testing.T) {
	input := `<box gap={1}>`

	lex := New(input)

	tokens := collectTokens(lex)

	expected := []TokenType{
		TOKEN_JSX_OPEN,      // <
		TOKEN_JSX_TAG,       // box
		TOKEN_JSX_ATTR_NAME, // gap
		TOKEN_JSX_EQUALS,    // =
		TOKEN_JSX_EXPR,      // 1
		TOKEN_JSX_CLOSE,     // >
		TOKEN_EOF,
	}

	assertTokenTypes(t, tokens, expected)

	// Check expression value
	if tokens[4].Value != "1" {
		t.Errorf("Expected expression '1', got %q", tokens[4].Value)
	}
}

func TestLexElementWithChildren(t *testing.T) {
	input := `<box>Hello World</box>`

	lex := New(input)

	tokens := collectTokens(lex)

	expected := []TokenType{
		TOKEN_JSX_OPEN,  // <
		TOKEN_JSX_TAG,   // box
		TOKEN_JSX_CLOSE, // >
		TOKEN_JSX_TEXT,  // Hello World
		TOKEN_JSX_OPEN,  // </
		TOKEN_JSX_TAG,   // box
		TOKEN_JSX_CLOSE, // >
		TOKEN_EOF,
	}

	assertTokenTypes(t, tokens, expected)

	// Check text content
	if tokens[3].Value != "Hello World" {
		t.Errorf("Expected text 'Hello World', got %q", tokens[3].Value)
	}
}

func TestLexElementWithExpressionChild(t *testing.T) {
	input := `<text>Hello {name}</text>`

	lex := New(input)

	tokens := collectTokens(lex)

	expected := []TokenType{
		TOKEN_JSX_OPEN,  // <
		TOKEN_JSX_TAG,   // text
		TOKEN_JSX_CLOSE, // >
		TOKEN_JSX_TEXT,  // Hello
		TOKEN_JSX_EXPR,  // name
		TOKEN_JSX_OPEN,  // </
		TOKEN_JSX_TAG,   // text
		TOKEN_JSX_CLOSE, // >
		TOKEN_EOF,
	}

	assertTokenTypes(t, tokens, expected)

	if tokens[4].Value != "name" {
		t.Errorf("Expected expression 'name', got %q", tokens[4].Value)
	}
}

func TestLexFragment(t *testing.T) {
	input := `<>Hello</>`

	lex := New(input)

	tokens := collectTokens(lex)

	expected := []TokenType{
		TOKEN_JSX_FRAG_OPEN,  // <>
		TOKEN_JSX_TEXT,       // Hello
		TOKEN_JSX_FRAG_CLOSE, // </>
		TOKEN_EOF,
	}

	assertTokenTypes(t, tokens, expected)
}

func TestLexGoCodeBeforeJSX(t *testing.T) {
	input := `package main

func App() VNode {
	return <box>Hello</box>
}`

	lex := New(input)

	tokens := collectTokens(lex)

	if tokens[0].Type != TOKEN_GO_CODE {
		t.Errorf("Expected GO_CODE first, got %v", tokens[0].Type)
	}

	// Should contain package, func, etc. before JSX
	if len(tokens[0].Value) < 30 {
		t.Errorf("Expected more Go code, got %q", tokens[0].Value)
	}
}

func TestLexNestedElements(t *testing.T) {
	input := `<box><text>Hi</text></box>`

	lex := New(input)

	tokens := collectTokens(lex)

	expected := []TokenType{
		TOKEN_JSX_OPEN,  // < (box)
		TOKEN_JSX_TAG,   // box
		TOKEN_JSX_CLOSE, // >
		TOKEN_JSX_OPEN,  // < (text)
		TOKEN_JSX_TAG,   // text
		TOKEN_JSX_CLOSE, // >
		TOKEN_JSX_TEXT,  // Hi
		TOKEN_JSX_OPEN,  // </ (text)
		TOKEN_JSX_TAG,   // text
		TOKEN_JSX_CLOSE, // >
		TOKEN_JSX_OPEN,  // </ (box)
		TOKEN_JSX_TAG,   // box
		TOKEN_JSX_CLOSE, // >
		TOKEN_EOF,
	}

	assertTokenTypes(t, tokens, expected)
}

func TestLexComplexExpression(t *testing.T) {
	input := `<box style={{color: "blue", padding: 10}}>`

	lex := New(input)

	tokens := collectTokens(lex)

	expected := []TokenType{
		TOKEN_JSX_OPEN,      // <
		TOKEN_JSX_TAG,       // box
		TOKEN_JSX_ATTR_NAME, // style
		TOKEN_JSX_EQUALS,    // =
		TOKEN_JSX_EXPR,      // {color: "blue", padding: 10}
		TOKEN_JSX_CLOSE,     // >
		TOKEN_EOF,
	}

	assertTokenTypes(t, tokens, expected)

	// Check expression preserves inner braces
	expr := tokens[4].Value
	if expr != `{color: "blue", padding: 10}` {
		t.Errorf("Expected expression with inner braces, got %q", expr)
	}
}

func TestLexDoesNotConfuseGoComparison(t *testing.T) {
	input := `package main

func foo() bool {
	return x < 10 && y > 5
}
`
	lex := New(input)
	tok := lex.NextToken()

	// Should lex entire thing as Go code, not try to parse < 10 as JSX
	if tok.Type != TOKEN_GO_CODE {
		t.Errorf("Expected GO_CODE, got %v", tok.Type)
	}

	tok = lex.NextToken()
	if tok.Type != TOKEN_EOF {
		t.Errorf("Expected EOF, got %v", tok.Type)
	}
}

func TestLexComponentElement(t *testing.T) {
	input := `<MyComponent foo="bar" />`

	lex := New(input)

	tokens := collectTokens(lex)

	expected := []TokenType{
		TOKEN_JSX_OPEN,      // <
		TOKEN_JSX_TAG,       // MyComponent
		TOKEN_JSX_ATTR_NAME, // foo
		TOKEN_JSX_EQUALS,    // =
		TOKEN_JSX_STRING,    // bar
		TOKEN_JSX_SLASH,     // /
		TOKEN_JSX_CLOSE,     // >
		TOKEN_EOF,
	}

	assertTokenTypes(t, tokens, expected)

	if tokens[1].Value != "MyComponent" {
		t.Errorf("Expected tag 'MyComponent', got %q", tokens[1].Value)
	}
}

// Helper functions

func collectTokens(lex *Lexer) []Token {
	var tokens []Token
	for {
		tok := lex.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == TOKEN_EOF || tok.Type == TOKEN_ERROR {
			break
		}
	}
	return tokens
}

func assertTokenTypes(t *testing.T, tokens []Token, expected []TokenType) {
	t.Helper()

	if len(tokens) != len(expected) {
		t.Errorf("Token count = %d, want %d", len(tokens), len(expected))
		for i, tok := range tokens {
			t.Logf("  [%d] %v", i, tok)
		}
		return
	}

	for i, tok := range tokens {
		if tok.Type != expected[i] {
			t.Errorf("tokens[%d].Type = %v, want %v", i, tok.Type, expected[i])
		}
	}
}
