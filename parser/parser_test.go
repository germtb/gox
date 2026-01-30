package parser

import (
	"testing"

	"github.com/germtb/gox/ast"
)

func TestParseSimpleElement(t *testing.T) {
	src := `<box></box>`

	file, err := Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(file.Nodes) != 1 {
		t.Fatalf("Expected 1 node, got %d", len(file.Nodes))
	}

	elem, ok := file.Nodes[0].(*ast.JSXElement)
	if !ok {
		t.Fatalf("Expected JSXElement, got %T", file.Nodes[0])
	}

	if elem.Tag != "box" {
		t.Errorf("Expected tag 'box', got %q", elem.Tag)
	}

	if elem.SelfClosing {
		t.Error("Expected non-self-closing element")
	}
}

func TestParseSelfClosingElement(t *testing.T) {
	src := `<input />`

	file, err := Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(file.Nodes) != 1 {
		t.Fatalf("Expected 1 node, got %d", len(file.Nodes))
	}

	elem, ok := file.Nodes[0].(*ast.JSXElement)
	if !ok {
		t.Fatalf("Expected JSXElement, got %T", file.Nodes[0])
	}

	if !elem.SelfClosing {
		t.Error("Expected self-closing element")
	}
}

func TestParseElementWithStringAttribute(t *testing.T) {
	src := `<box direction="row"></box>`

	file, err := Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	elem := file.Nodes[0].(*ast.JSXElement)

	if len(elem.Attributes) != 1 {
		t.Fatalf("Expected 1 attribute, got %d", len(elem.Attributes))
	}

	attr, ok := elem.Attributes[0].(*ast.StringAttribute)
	if !ok {
		t.Fatalf("Expected StringAttribute, got %T", elem.Attributes[0])
	}

	if attr.Key != "direction" {
		t.Errorf("Expected key 'direction', got %q", attr.Key)
	}
	if attr.Value != "row" {
		t.Errorf("Expected value 'row', got %q", attr.Value)
	}
}

func TestParseElementWithExpressionAttribute(t *testing.T) {
	src := `<box gap={1}></box>`

	file, err := Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	elem := file.Nodes[0].(*ast.JSXElement)

	if len(elem.Attributes) != 1 {
		t.Fatalf("Expected 1 attribute, got %d", len(elem.Attributes))
	}

	attr, ok := elem.Attributes[0].(*ast.ExpressionAttribute)
	if !ok {
		t.Fatalf("Expected ExpressionAttribute, got %T", elem.Attributes[0])
	}

	if attr.Key != "gap" {
		t.Errorf("Expected key 'gap', got %q", attr.Key)
	}
	if attr.Expression != "1" {
		t.Errorf("Expected expression '1', got %q", attr.Expression)
	}
}

func TestParseElementWithTextChild(t *testing.T) {
	src := `<text>Hello World</text>`

	file, err := Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	elem := file.Nodes[0].(*ast.JSXElement)

	if len(elem.Children) != 1 {
		t.Fatalf("Expected 1 child, got %d", len(elem.Children))
	}

	text, ok := elem.Children[0].(*ast.JSXText)
	if !ok {
		t.Fatalf("Expected JSXText, got %T", elem.Children[0])
	}

	if text.Value != "Hello World" {
		t.Errorf("Expected 'Hello World', got %q", text.Value)
	}
}

func TestParseElementWithExpressionChild(t *testing.T) {
	src := `<text>Hello {name}</text>`

	file, err := Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	elem := file.Nodes[0].(*ast.JSXElement)

	if len(elem.Children) != 2 {
		t.Fatalf("Expected 2 children, got %d", len(elem.Children))
	}

	text, ok := elem.Children[0].(*ast.JSXText)
	if !ok {
		t.Fatalf("Expected JSXText, got %T", elem.Children[0])
	}
	if text.Value != "Hello " {
		t.Errorf("Expected 'Hello ', got %q", text.Value)
	}

	expr, ok := elem.Children[1].(*ast.JSXExpression)
	if !ok {
		t.Fatalf("Expected JSXExpression, got %T", elem.Children[1])
	}
	if expr.Expression != "name" {
		t.Errorf("Expected expression 'name', got %q", expr.Expression)
	}
}

func TestParseNestedElements(t *testing.T) {
	src := `<box><text>Hi</text></box>`

	file, err := Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	box := file.Nodes[0].(*ast.JSXElement)
	if box.Tag != "box" {
		t.Errorf("Expected 'box', got %q", box.Tag)
	}

	if len(box.Children) != 1 {
		t.Fatalf("Expected 1 child, got %d", len(box.Children))
	}

	text, ok := box.Children[0].(*ast.JSXElement)
	if !ok {
		t.Fatalf("Expected JSXElement, got %T", box.Children[0])
	}
	if text.Tag != "text" {
		t.Errorf("Expected 'text', got %q", text.Tag)
	}
}

func TestParseFragment(t *testing.T) {
	src := `<>Hello</>`

	file, err := Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(file.Nodes) != 1 {
		t.Fatalf("Expected 1 node, got %d", len(file.Nodes))
	}

	frag, ok := file.Nodes[0].(*ast.JSXFragment)
	if !ok {
		t.Fatalf("Expected JSXFragment, got %T", file.Nodes[0])
	}

	if len(frag.Children) != 1 {
		t.Fatalf("Expected 1 child, got %d", len(frag.Children))
	}
}

func TestParseGoCodeBeforeJSX(t *testing.T) {
	src := `package main

func App() VNode {
	return <box>Hello</box>
}`

	file, err := Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(file.Nodes) < 2 {
		t.Fatalf("Expected at least 2 nodes, got %d", len(file.Nodes))
	}

	// First node should be Go code
	goCode, ok := file.Nodes[0].(*ast.GoCode)
	if !ok {
		t.Fatalf("Expected GoCode, got %T", file.Nodes[0])
	}
	if len(goCode.Value) < 20 {
		t.Errorf("Expected more Go code, got %q", goCode.Value)
	}

	// Second node should be JSX
	_, ok = file.Nodes[1].(*ast.JSXElement)
	if !ok {
		t.Fatalf("Expected JSXElement, got %T", file.Nodes[1])
	}
}

func TestParseSpreadAttribute(t *testing.T) {
	src := `<box {...props}></box>`

	file, err := Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	elem := file.Nodes[0].(*ast.JSXElement)

	if len(elem.Attributes) != 1 {
		t.Fatalf("Expected 1 attribute, got %d", len(elem.Attributes))
	}

	spread, ok := elem.Attributes[0].(*ast.SpreadAttribute)
	if !ok {
		t.Fatalf("Expected SpreadAttribute, got %T", elem.Attributes[0])
	}

	if spread.Expression != "props" {
		t.Errorf("Expected expression 'props', got %q", spread.Expression)
	}
}

func TestParseMultipleAttributes(t *testing.T) {
	src := `<box direction="row" gap={1} wrap></box>`

	file, err := Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	elem := file.Nodes[0].(*ast.JSXElement)

	if len(elem.Attributes) != 3 {
		t.Fatalf("Expected 3 attributes, got %d", len(elem.Attributes))
	}

	// First: string attribute
	attr1, ok := elem.Attributes[0].(*ast.StringAttribute)
	if !ok {
		t.Fatalf("Expected StringAttribute, got %T", elem.Attributes[0])
	}
	if attr1.Key != "direction" || attr1.Value != "row" {
		t.Errorf("Unexpected first attribute: %+v", attr1)
	}

	// Second: expression attribute
	attr2, ok := elem.Attributes[1].(*ast.ExpressionAttribute)
	if !ok {
		t.Fatalf("Expected ExpressionAttribute, got %T", elem.Attributes[1])
	}
	if attr2.Key != "gap" || attr2.Expression != "1" {
		t.Errorf("Unexpected second attribute: %+v", attr2)
	}

	// Third: boolean attribute
	attr3, ok := elem.Attributes[2].(*ast.ExpressionAttribute)
	if !ok {
		t.Fatalf("Expected ExpressionAttribute (boolean), got %T", elem.Attributes[2])
	}
	if attr3.Key != "wrap" || attr3.Expression != "true" {
		t.Errorf("Unexpected third attribute: %+v", attr3)
	}
}

func TestParseComplexNestedStructure(t *testing.T) {
	src := `<box direction="row">
	<text style={{color: "blue"}}>Hello</text>
	<text>{name}</text>
</box>`

	file, err := Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	box := file.Nodes[0].(*ast.JSXElement)
	if box.Tag != "box" {
		t.Errorf("Expected 'box', got %q", box.Tag)
	}

	// Should have whitespace text and two text elements
	// (Exact count depends on whitespace handling)
	textElements := 0
	for _, child := range box.Children {
		if elem, ok := child.(*ast.JSXElement); ok && elem.Tag == "text" {
			textElements++
		}
	}
	if textElements != 2 {
		t.Errorf("Expected 2 text elements, got %d", textElements)
	}
}
