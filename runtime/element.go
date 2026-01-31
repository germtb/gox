package runtime

// Element creates a VNode for an element (intrinsic or component).
// typ can be a string (for intrinsic elements like "box", "text")
// or a Component function.
func Element(typ any, props Props, children ...VNode) VNode {
	if props == nil {
		props = Props{}
	}
	return VNode{
		Type:     typ,
		Props:    props,
		Children: children,
	}
}

// E is a shorthand alias for Element.
func E(typ any, props Props, children ...VNode) VNode {
	return Element(typ, props, children...)
}
