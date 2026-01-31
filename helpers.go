package gox

import "fmt"

// Text creates a text VNode.
func Text(content string) VNode {
	return VNode{
		Type:  TextNodeType,
		Props: Props{"content": content},
	}
}

// V converts an arbitrary value to a VNode.
// If the value is already a VNode, it's returned as-is.
// If it's a string, it's wrapped as a Text node.
// If it's a []VNode, it's wrapped as a Fragment.
// Numeric types and booleans are converted to their string representation.
// Panics for unsupported types (channels, functions, etc.).
func V(value any) VNode {
	switch v := value.(type) {
	case VNode:
		return v
	case string:
		return Text(v)
	case []VNode:
		return Fragment(v...)
	case nil:
		return Empty()
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, bool:
		return Text(fmt.Sprint(v))
	default:
		// Check for common unsupported types that indicate a bug
		panic(fmt.Sprintf("gox: cannot convert %T to VNode - use gox.Text() for strings or return a VNode from your expression", value))
	}
}

// Fragment wraps multiple children without a parent element.
func Fragment(children ...VNode) VNode {
	return VNode{
		Type:     FragmentNodeType,
		Children: children,
	}
}

// When returns child if condition is true, else empty VNode.
// Useful for conditional rendering: {gox.When(showExtra, <Extra />)}
func When(condition bool, child VNode) VNode {
	if condition {
		return child
	}
	return Empty()
}

// WhenElse returns ifTrue if condition is true, else ifFalse.
func WhenElse(condition bool, ifTrue, ifFalse VNode) VNode {
	if condition {
		return ifTrue
	}
	return ifFalse
}

// Map applies a function to each element and returns the resulting VNodes.
// Useful for rendering lists: {gox.Map(items, func(item Item) VNode { return <ItemView item={item} /> })}
func Map[T any](items []T, fn func(T) VNode) []VNode {
	result := make([]VNode, len(items))
	for i, item := range items {
		result[i] = fn(item)
	}
	return result
}

// MapIndex applies a function with index to each element and returns the resulting VNodes.
func MapIndex[T any](items []T, fn func(int, T) VNode) []VNode {
	result := make([]VNode, len(items))
	for i, item := range items {
		result[i] = fn(i, item)
	}
	return result
}

// Spread expands a slice of VNodes into children.
// Useful when you have a []VNode and need to pass as children.
func Spread(nodes []VNode) VNode {
	return Fragment(nodes...)
}
