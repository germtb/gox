package ast

import "testing"

func TestPositionIsValid(t *testing.T) {
	tests := []struct {
		name     string
		pos      Position
		expected bool
	}{
		{"zero position", Position{}, false},
		{"valid position", Position{Offset: 0, Line: 1, Column: 1}, true},
		{"zero line", Position{Offset: 10, Line: 0, Column: 5}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pos.IsValid(); got != tt.expected {
				t.Errorf("Position.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRangeIsValid(t *testing.T) {
	tests := []struct {
		name     string
		r        Range
		expected bool
	}{
		{"zero range", Range{}, false},
		{"valid range", Range{
			Start: Position{Offset: 0, Line: 1, Column: 1},
			End:   Position{Offset: 10, Line: 1, Column: 11},
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.IsValid(); got != tt.expected {
				t.Errorf("Range.IsValid() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestJSXElementImplementsInterfaces(t *testing.T) {
	elem := &JSXElement{
		Tag:         "box",
		Attributes:  nil,
		Children:    nil,
		SelfClosing: false,
	}

	// Verify JSXElement implements Node
	var _ Node = elem

	// Verify JSXElement implements JSXChild
	var _ JSXChild = elem
}

func TestAttributeTypes(t *testing.T) {
	// Verify all attribute types implement Attribute interface
	var _ Attribute = &StringAttribute{Key: "id", Value: "test"}
	var _ Attribute = &ExpressionAttribute{Key: "onClick", Expression: "handleClick"}
}

func TestJSXChildTypes(t *testing.T) {
	// Verify all JSXChild types implement the interface
	var _ JSXChild = &JSXText{Value: "Hello"}
	var _ JSXChild = &JSXExpression{Expression: "name"}
	var _ JSXChild = &JSXElement{Tag: "span"}
	var _ JSXChild = &JSXFragment{}
}

func TestGoCodeImplementsNode(t *testing.T) {
	code := &GoCode{Value: "func main() {}"}
	var _ Node = code
}
