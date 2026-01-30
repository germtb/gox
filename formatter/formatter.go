// Package formatter provides formatting for .gox files.
package formatter

import (
	"bytes"
	"go/format"
	"strings"
	"unicode"

	"github.com/germtb/gox/ast"
)

// Options configures the formatter.
type Options struct {
	// TabWidth is the number of spaces per tab (for display purposes).
	TabWidth int
	// UseTabs uses tabs instead of spaces.
	UseTabs bool
	// MaxLineLength is the target max line length before wrapping attributes.
	MaxLineLength int
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() *Options {
	return &Options{
		TabWidth:      4,
		UseTabs:       true,
		MaxLineLength: 100,
	}
}

// Formatter formats .gox files.
type Formatter struct {
	opts   *Options
	buf    bytes.Buffer
	indent int
}

// New creates a new Formatter.
func New(opts *Options) *Formatter {
	if opts == nil {
		opts = DefaultOptions()
	}
	return &Formatter{opts: opts}
}

// Format formats a parsed .gox file.
func Format(file *ast.GoxFile, opts *Options) ([]byte, error) {
	f := New(opts)
	return f.Format(file)
}

// Format formats the AST back to source code.
func (f *Formatter) Format(file *ast.GoxFile) ([]byte, error) {
	f.buf.Reset()
	f.indent = 0

	for _, node := range file.Nodes {
		f.formatNode(node)
	}

	return f.buf.Bytes(), nil
}

// formatNode formats a single node.
func (f *Formatter) formatNode(node ast.Node) {
	switch n := node.(type) {
	case *ast.GoCode:
		f.formatGoCode(n)
	case *ast.JSXElement:
		f.formatJSXElement(n, false)
	case *ast.JSXFragment:
		f.formatJSXFragment(n, false)
	}
}

// formatGoCode formats Go code, preserving it mostly as-is.
// We only format complete Go files/declarations, not partial code.
func (f *Formatter) formatGoCode(code *ast.GoCode) {
	value := code.Value

	// Detect the base indentation from the code
	f.indent = f.detectIndent(value)

	// Try to format as a complete Go source (package + imports + decls)
	if strings.HasPrefix(strings.TrimSpace(value), "package ") {
		formatted, err := format.Source([]byte(value))
		if err == nil {
			f.buf.Write(formatted)
			return
		}
	}

	// For partial Go code (function bodies, etc.), normalize trailing whitespace
	// This handles cases like "return    " -> "return "
	value = normalizeTrailingWhitespace(value)
	f.buf.WriteString(value)
}

// normalizeTrailingWhitespace normalizes whitespace at the end of Go code.
// It collapses multiple trailing spaces/tabs on the last line to a single space,
// while preserving newlines and indentation structure.
func normalizeTrailingWhitespace(code string) string {
	if code == "" {
		return code
	}

	// Find the last newline
	lastNewline := strings.LastIndex(code, "\n")

	if lastNewline == -1 {
		// No newlines - just trim trailing whitespace and add single space if needed
		trimmed := strings.TrimRight(code, " \t")
		if trimmed != code {
			// There was trailing whitespace - normalize to single space
			return trimmed + " "
		}
		return code
	}

	// Split into prefix (up to and including last newline) and suffix (after last newline)
	prefix := code[:lastNewline+1]
	suffix := code[lastNewline+1:]

	// Normalize trailing whitespace in suffix (the last line)
	// Keep leading tabs (indentation) but normalize trailing spaces
	trimmedSuffix := strings.TrimRight(suffix, " \t")
	leadingWhitespace := ""
	for _, r := range suffix {
		if r == '\t' || r == ' ' {
			leadingWhitespace += string(r)
		} else {
			break
		}
	}

	if trimmedSuffix == "" {
		// Last line is only whitespace (indentation) - this is the line before JSX
		// Check if there's content before that we need to normalize
		return prefix + suffix
	}

	// There's content on the last line - normalize trailing whitespace
	content := strings.TrimLeft(trimmedSuffix, " \t")
	if len(suffix) > len(trimmedSuffix) {
		// There was trailing whitespace - normalize to single space
		return prefix + leadingWhitespace + content + " "
	}

	return code
}

// formatJSXElement formats a JSX element.
func (f *Formatter) formatJSXElement(elem *ast.JSXElement, isChild bool) {
	// Determine if we should format inline or multiline
	inline := f.shouldInline(elem)

	// Opening tag
	f.buf.WriteString("<")
	f.buf.WriteString(elem.Tag)

	// Attributes
	if len(elem.Attributes) > 0 {
		if inline || len(elem.Attributes) <= 2 {
			// Inline attributes
			for _, attr := range elem.Attributes {
				f.buf.WriteString(" ")
				f.formatAttribute(attr)
			}
		} else {
			// Multiline attributes
			f.indent++
			for _, attr := range elem.Attributes {
				f.buf.WriteString("\n")
				f.writeIndent()
				f.formatAttribute(attr)
			}
			f.indent--
		}
	}

	// Self-closing or with children
	if elem.SelfClosing {
		f.buf.WriteString(" />")
	} else if len(elem.Children) == 0 {
		f.buf.WriteString("></")
		f.buf.WriteString(elem.Tag)
		f.buf.WriteString(">")
	} else if inline {
		// Inline children - render on same line
		f.buf.WriteString(">")
		for _, child := range elem.Children {
			f.formatJSXChildInline(child)
		}
		f.buf.WriteString("</")
		f.buf.WriteString(elem.Tag)
		f.buf.WriteString(">")
	} else {
		// Multiline children
		f.buf.WriteString(">")
		f.indent++
		for _, child := range elem.Children {
			f.formatJSXChild(child)
		}
		f.indent--
		f.buf.WriteString("\n")
		f.writeIndent()
		f.buf.WriteString("</")
		f.buf.WriteString(elem.Tag)
		f.buf.WriteString(">")
	}
}

// formatJSXFragment formats a JSX fragment.
func (f *Formatter) formatJSXFragment(frag *ast.JSXFragment, isChild bool) {
	f.buf.WriteString("<>")

	if len(frag.Children) > 0 {
		f.indent++
		for _, child := range frag.Children {
			f.formatJSXChild(child)
		}
		f.indent--
		f.buf.WriteString("\n")
		f.writeIndent()
	}

	f.buf.WriteString("</>")
}

// formatJSXChild formats a JSX child element (multiline).
func (f *Formatter) formatJSXChild(child ast.JSXChild) {
	switch c := child.(type) {
	case *ast.JSXText:
		trimmed := strings.TrimSpace(c.Value)
		if trimmed != "" {
			f.buf.WriteString("\n")
			f.writeIndent()
			f.buf.WriteString(trimmed)
		}
	case *ast.JSXExpression:
		f.buf.WriteString("\n")
		f.writeIndent()
		f.buf.WriteString("{")
		f.buf.WriteString(strings.TrimSpace(c.Expression))
		f.buf.WriteString("}")
	case *ast.JSXElement:
		f.buf.WriteString("\n")
		f.writeIndent()
		f.formatJSXElement(c, true)
	case *ast.JSXFragment:
		f.buf.WriteString("\n")
		f.writeIndent()
		f.formatJSXFragment(c, true)
	}
}

// formatJSXChildInline formats a JSX child inline (no newlines).
func (f *Formatter) formatJSXChildInline(child ast.JSXChild) {
	switch c := child.(type) {
	case *ast.JSXText:
		// Normalize whitespace: collapse multiple spaces/tabs/newlines to single space
		// but preserve leading/trailing spaces if they exist
		text := c.Value
		// Replace any whitespace sequence with a single space
		var result strings.Builder
		inWhitespace := false
		for _, r := range text {
			if unicode.IsSpace(r) {
				if !inWhitespace {
					result.WriteByte(' ')
					inWhitespace = true
				}
			} else {
				result.WriteRune(r)
				inWhitespace = false
			}
		}
		normalized := result.String()
		if normalized != "" && normalized != " " {
			f.buf.WriteString(normalized)
		} else if normalized == " " {
			// Preserve single space between expressions
			f.buf.WriteString(" ")
		}
	case *ast.JSXExpression:
		f.buf.WriteString("{")
		f.buf.WriteString(strings.TrimSpace(c.Expression))
		f.buf.WriteString("}")
	case *ast.JSXElement:
		f.formatJSXElement(c, true)
	case *ast.JSXFragment:
		f.formatJSXFragment(c, true)
	}
}

// formatAttribute formats a single attribute.
func (f *Formatter) formatAttribute(attr ast.Attribute) {
	switch a := attr.(type) {
	case *ast.StringAttribute:
		f.buf.WriteString(a.Key)
		f.buf.WriteString("=\"")
		f.buf.WriteString(a.Value)
		f.buf.WriteString("\"")
	case *ast.ExpressionAttribute:
		f.buf.WriteString(a.Key)
		f.buf.WriteString("={")
		f.buf.WriteString(strings.TrimSpace(a.Expression))
		f.buf.WriteString("}")
	case *ast.SpreadAttribute:
		f.buf.WriteString("{...")
		f.buf.WriteString(strings.TrimSpace(a.Expression))
		f.buf.WriteString("}")
	}
}

// shouldInline determines if an element should be formatted inline.
func (f *Formatter) shouldInline(elem *ast.JSXElement) bool {
	// Self-closing elements with few attributes
	if elem.SelfClosing && len(elem.Attributes) <= 3 {
		return true
	}

	// Elements with only simple inline content (text + expressions, no nested elements)
	if len(elem.Children) > 0 {
		hasNestedElements := false
		totalLength := 0
		for _, child := range elem.Children {
			switch c := child.(type) {
			case *ast.JSXElement, *ast.JSXFragment:
				hasNestedElements = true
			case *ast.JSXText:
				totalLength += len(strings.TrimSpace(c.Value))
			case *ast.JSXExpression:
				totalLength += len(c.Expression) + 2 // {}
			}
		}
		// If no nested elements and content is short, inline it
		if !hasNestedElements && totalLength < 60 {
			return true
		}
	}

	// Elements with only simple text content
	if len(elem.Children) == 1 {
		if text, ok := elem.Children[0].(*ast.JSXText); ok {
			trimmed := strings.TrimSpace(text.Value)
			if len(trimmed) < 40 && !strings.Contains(trimmed, "\n") {
				return true
			}
		}
	}

	// Empty elements
	if len(elem.Children) == 0 && len(elem.Attributes) <= 3 {
		return true
	}

	return false
}

// writeIndent writes the current indentation.
func (f *Formatter) writeIndent() {
	if f.opts.UseTabs {
		for i := 0; i < f.indent; i++ {
			f.buf.WriteByte('\t')
		}
	} else {
		for i := 0; i < f.indent*f.opts.TabWidth; i++ {
			f.buf.WriteByte(' ')
		}
	}
}

// detectIndent detects the indentation level from a Go code snippet.
// It looks at the last line to determine the current indent level.
func (f *Formatter) detectIndent(code string) int {
	lines := strings.Split(code, "\n")
	if len(lines) == 0 {
		return 0
	}

	// Look at the last non-empty line
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Count leading tabs
		tabs := 0
		for _, r := range line {
			if r == '\t' {
				tabs++
			} else {
				break
			}
		}
		return tabs
	}
	return 0
}
