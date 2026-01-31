// Package ast defines the AST types for gox files.
package ast

// GoxFile represents a complete .gox file.
type GoxFile struct {
	Package    string
	Imports    []Import
	Nodes      []Node // Go code + JSX intermixed
	SourcePath string
}

// Import represents a Go import statement.
type Import struct {
	Alias string // Optional alias (e.g., "ui" in `import ui "myapp/ui"`)
	Path  string // Import path (e.g., "myapp/ui")
	Range Range
}

// Node is the interface for all nodes in a gox file.
type Node interface {
	node()
	GetRange() Range
}

// JSXElement represents <tag ...>...</tag> or <tag ... />.
type JSXElement struct {
	Range       Range
	Tag         string // "box", "text", or component name
	Attributes  []Attribute
	Children    []JSXChild
	SelfClosing bool
}

func (*JSXElement) node()             {}
func (e *JSXElement) GetRange() Range { return e.Range }
func (*JSXElement) jsxChildNode()     {}

// Attribute can be string or expression.
type Attribute interface {
	attributeNode()
	GetRange() Range
}

// StringAttribute represents key="value".
type StringAttribute struct {
	Key   string
	Value string
	Range Range
}

func (*StringAttribute) attributeNode()    {}
func (a *StringAttribute) GetRange() Range { return a.Range }

// ExpressionAttribute represents key={expression}.
type ExpressionAttribute struct {
	Key        string
	Expression string
	Range      Range
}

func (*ExpressionAttribute) attributeNode()    {}
func (a *ExpressionAttribute) GetRange() Range { return a.Range }

// JSXChild can be text, expression, or nested element.
type JSXChild interface {
	jsxChildNode()
	GetRange() Range
}

// JSXText represents text content between tags.
type JSXText struct {
	Value string
	Range Range
}

func (*JSXText) jsxChildNode()     {}
func (t *JSXText) GetRange() Range { return t.Range }

// JSXExpression represents {expression} within JSX.
type JSXExpression struct {
	Expression string
	Range      Range
}

func (*JSXExpression) jsxChildNode()     {}
func (e *JSXExpression) GetRange() Range { return e.Range }

// GoCode represents pass-through Go code.
type GoCode struct {
	Value string
	Range Range
}

func (*GoCode) node()             {}
func (c *GoCode) GetRange() Range { return c.Range }

// JSXFragment represents <>...</> (fragment without tag).
type JSXFragment struct {
	Range    Range
	Children []JSXChild
}

func (*JSXFragment) node()             {}
func (f *JSXFragment) GetRange() Range { return f.Range }
func (*JSXFragment) jsxChildNode()     {}
