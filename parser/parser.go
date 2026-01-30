// Package parser parses gox files into an AST.
package parser

import (
	"fmt"

	"github.com/germtb/gox/ast"
	"github.com/germtb/gox/lexer"
)

// Parser parses gox source files.
type Parser struct {
	lex      *lexer.Lexer
	tok      lexer.Token
	peekTok  lexer.Token
	hasPeek  bool
	errors   []error
	filename string
}

// New creates a new Parser.
func New(filename string, src []byte) *Parser {
	return &Parser{
		lex:      lexer.New(string(src)),
		filename: filename,
	}
}

// Parse parses a gox file and returns the AST.
func Parse(filename string, src []byte) (*ast.GoxFile, error) {
	p := New(filename, src)
	return p.Parse()
}

// Parse parses the source and returns a GoxFile AST.
func (p *Parser) Parse() (*ast.GoxFile, error) {
	p.advance() // load first token

	file := &ast.GoxFile{
		SourcePath: p.filename,
		Nodes:      []ast.Node{},
	}

	for p.tok.Type != lexer.TOKEN_EOF {
		node := p.parseNode()
		if node != nil {
			file.Nodes = append(file.Nodes, node)
		}
	}

	if len(p.errors) > 0 {
		return file, p.errors[0]
	}
	return file, nil
}

// parseNode parses a single top-level node (Go code or JSX element).
func (p *Parser) parseNode() ast.Node {
	switch p.tok.Type {
	case lexer.TOKEN_GO_CODE:
		return p.parseGoCode()
	case lexer.TOKEN_JSX_OPEN, lexer.TOKEN_JSX_FRAG_OPEN:
		return p.parseJSXElement()
	case lexer.TOKEN_EOF:
		return nil
	default:
		p.error("unexpected token: %v", p.tok)
		p.advance()
		return nil
	}
}

// parseGoCode parses a Go code block.
func (p *Parser) parseGoCode() *ast.GoCode {
	code := &ast.GoCode{
		Value: p.tok.Value,
		Range: p.tokenRange(),
	}
	p.advance()
	return code
}

// parseJSXElement parses a JSX element or fragment.
func (p *Parser) parseJSXElement() ast.Node {
	startRange := p.tokenRange()

	if p.tok.Type == lexer.TOKEN_JSX_FRAG_OPEN {
		return p.parseJSXFragment(startRange)
	}

	// Parse opening tag: <tagname
	if p.tok.Type != lexer.TOKEN_JSX_OPEN {
		p.error("expected '<', got %v", p.tok)
		return nil
	}
	p.advance() // consume <

	// Get tag name
	if p.tok.Type != lexer.TOKEN_JSX_TAG {
		p.error("expected tag name, got %v", p.tok)
		return nil
	}
	tagName := p.tok.Value
	p.advance()

	// Parse attributes
	attrs := p.parseJSXAttributes()

	// Check for self-closing or children
	elem := &ast.JSXElement{
		Tag:        tagName,
		Attributes: attrs,
		Range:      startRange,
	}

	if p.tok.Type == lexer.TOKEN_JSX_SLASH {
		// Self-closing: />
		elem.SelfClosing = true
		p.advance() // consume /
		if p.tok.Type != lexer.TOKEN_JSX_CLOSE {
			p.error("expected '>' after '/', got %v", p.tok)
		} else {
			p.advance() // consume >
		}
		elem.Range.End = p.prevPosition()
		return elem
	}

	if p.tok.Type != lexer.TOKEN_JSX_CLOSE {
		p.error("expected '>' or '/>', got %v", p.tok)
		return elem
	}
	p.advance() // consume >

	// Parse children
	elem.Children = p.parseJSXChildren(tagName)

	// Parse closing tag: </tagname>
	if p.tok.Type == lexer.TOKEN_JSX_OPEN {
		p.advance() // consume </
		if p.tok.Type == lexer.TOKEN_JSX_TAG {
			closeTag := p.tok.Value
			if closeTag != tagName {
				p.error("mismatched closing tag: expected </%s>, got </%s>", tagName, closeTag)
			}
			p.advance()
		}
		if p.tok.Type == lexer.TOKEN_JSX_CLOSE {
			p.advance() // consume >
		}
	}

	elem.Range.End = p.prevPosition()
	return elem
}

// parseJSXFragment parses a JSX fragment <>...</>.
func (p *Parser) parseJSXFragment(startRange ast.Range) *ast.JSXFragment {
	frag := &ast.JSXFragment{
		Range: startRange,
	}
	p.advance() // consume <>

	// Parse children until </>
	frag.Children = p.parseJSXChildren("")

	// Consume </>
	if p.tok.Type == lexer.TOKEN_JSX_FRAG_CLOSE {
		p.advance()
	}

	frag.Range.End = p.prevPosition()
	return frag
}

// parseJSXAttributes parses JSX attributes until we hit > or />.
func (p *Parser) parseJSXAttributes() []ast.Attribute {
	var attrs []ast.Attribute

	for {
		switch p.tok.Type {
		case lexer.TOKEN_JSX_CLOSE, lexer.TOKEN_JSX_SLASH, lexer.TOKEN_EOF:
			return attrs

		case lexer.TOKEN_JSX_ATTR_NAME:
			attr := p.parseJSXAttribute()
			if attr != nil {
				attrs = append(attrs, attr)
			}

		case lexer.TOKEN_JSX_EXPR:
			// Spread attribute: {...expr}
			if len(p.tok.Value) > 3 && p.tok.Value[:3] == "..." {
				attrs = append(attrs, &ast.SpreadAttribute{
					Expression: p.tok.Value[3:],
					Range:      p.tokenRange(),
				})
			}
			p.advance()

		default:
			p.error("unexpected token in attributes: %v", p.tok)
			p.advance()
		}
	}
}

// parseJSXAttribute parses a single JSX attribute.
func (p *Parser) parseJSXAttribute() ast.Attribute {
	if p.tok.Type != lexer.TOKEN_JSX_ATTR_NAME {
		return nil
	}

	name := p.tok.Value
	startRange := p.tokenRange()
	p.advance()

	// Check for = value
	if p.tok.Type != lexer.TOKEN_JSX_EQUALS {
		// Boolean attribute (no value)
		return &ast.ExpressionAttribute{
			Key:        name,
			Expression: "true",
			Range:      startRange,
		}
	}
	p.advance() // consume =

	// Parse value
	switch p.tok.Type {
	case lexer.TOKEN_JSX_STRING:
		attr := &ast.StringAttribute{
			Key:   name,
			Value: p.tok.Value,
			Range: startRange,
		}
		attr.Range.End = p.tokenRange().End
		p.advance()
		return attr

	case lexer.TOKEN_JSX_EXPR:
		attr := &ast.ExpressionAttribute{
			Key:        name,
			Expression: p.tok.Value,
			Range:      startRange,
		}
		attr.Range.End = p.tokenRange().End
		p.advance()
		return attr

	default:
		p.error("expected string or expression for attribute value, got %v", p.tok)
		return nil
	}
}

// parseJSXChildren parses children until closing tag or fragment close.
func (p *Parser) parseJSXChildren(parentTag string) []ast.JSXChild {
	var children []ast.JSXChild

	for {
		switch p.tok.Type {
		case lexer.TOKEN_EOF:
			return children

		case lexer.TOKEN_JSX_FRAG_CLOSE:
			// End of fragment
			return children

		case lexer.TOKEN_JSX_OPEN:
			// Could be closing tag or nested element
			if p.isClosingTag() {
				return children
			}
			// Nested element
			child := p.parseJSXElement()
			if child != nil {
				if elem, ok := child.(*ast.JSXElement); ok {
					children = append(children, elem)
				}
			}

		case lexer.TOKEN_JSX_FRAG_OPEN:
			// Nested fragment
			child := p.parseJSXElement()
			if child != nil {
				if frag, ok := child.(*ast.JSXFragment); ok {
					children = append(children, frag)
				}
			}

		case lexer.TOKEN_JSX_TEXT:
			text := &ast.JSXText{
				Value: p.tok.Value,
				Range: p.tokenRange(),
			}
			children = append(children, text)
			p.advance()

		case lexer.TOKEN_JSX_EXPR:
			expr := &ast.JSXExpression{
				Expression: p.tok.Value,
				Range:      p.tokenRange(),
			}
			children = append(children, expr)
			p.advance()

		default:
			p.error("unexpected token in children: %v", p.tok)
			p.advance()
		}
	}
}

// isClosingTag checks if current position is a closing tag </
func (p *Parser) isClosingTag() bool {
	return p.tok.Type == lexer.TOKEN_JSX_OPEN && len(p.tok.Value) == 2 && p.tok.Value == "</"
}

// Helper methods

func (p *Parser) advance() {
	if p.hasPeek {
		p.tok = p.peekTok
		p.hasPeek = false
	} else {
		p.tok = p.lex.NextToken()
	}
}

func (p *Parser) peek() lexer.Token {
	if !p.hasPeek {
		p.peekTok = p.lex.NextToken()
		p.hasPeek = true
	}
	return p.peekTok
}

func (p *Parser) error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	err := fmt.Errorf("%s:%d:%d: %s", p.filename, p.tok.Line, p.tok.Column, msg)
	p.errors = append(p.errors, err)
}

func (p *Parser) tokenRange() ast.Range {
	return ast.Range{
		Start: ast.Position{
			Offset: p.tok.Offset,
			Line:   p.tok.Line,
			Column: p.tok.Column,
		},
		End: ast.Position{
			Offset: p.tok.Offset + len(p.tok.Value),
			Line:   p.tok.Line,
			Column: p.tok.Column + len(p.tok.Value),
		},
	}
}

func (p *Parser) prevPosition() ast.Position {
	// This is approximate; for exact tracking we'd need to store previous token
	return ast.Position{
		Offset: p.tok.Offset,
		Line:   p.tok.Line,
		Column: p.tok.Column,
	}
}
