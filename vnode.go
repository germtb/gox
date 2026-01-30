// Package gox provides JSX-like syntax for Go and the core types for virtual DOM trees.
package gox

// VNode is the core tree node type.
type VNode struct {
	Type     any // string for intrinsic elements, Component for components
	Props    Props
	Children []VNode
}

// Props is a flexible property map.
type Props map[string]any

// Component is a function that returns a VNode.
type Component func(props Props) VNode

// NodeType constants for special node types.
const (
	TextNodeType     = "__text__"
	FragmentNodeType = "__fragment__"
)

// IsText returns true if this VNode is a text node.
func (v VNode) IsText() bool {
	s, ok := v.Type.(string)
	return ok && s == TextNodeType
}

// IsFragment returns true if this VNode is a fragment.
func (v VNode) IsFragment() bool {
	s, ok := v.Type.(string)
	return ok && s == FragmentNodeType
}

// IsComponent returns true if this VNode represents a component.
func (v VNode) IsComponent() bool {
	_, ok := v.Type.(Component)
	return ok
}

// GetTextContent returns the text content if this is a text node.
func (v VNode) GetTextContent() (string, bool) {
	if !v.IsText() {
		return "", false
	}
	if content, ok := v.Props["content"].(string); ok {
		return content, true
	}
	return "", false
}

// Empty returns an empty VNode.
func Empty() VNode {
	return VNode{}
}

// IsEmpty returns true if this VNode is empty/nil.
func (v VNode) IsEmpty() bool {
	return v.Type == nil && v.Props == nil && v.Children == nil
}
