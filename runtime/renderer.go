package runtime

// Renderer is the interface for connecting VNode trees to actual implementations.
// Users implement this interface to bridge gox output to their tree system
// (e.g., treeli TUI, HTML DOM, custom tree structures).
type Renderer interface {
	// Render processes a VNode tree and produces output.
	Render(vnode VNode) error
}

// RenderFunc is a function type that implements Renderer.
type RenderFunc func(VNode) error

// Render implements the Renderer interface.
func (f RenderFunc) Render(vnode VNode) error {
	return f(vnode)
}

// Walker provides a way to traverse VNode trees.
type Walker interface {
	// Walk is called for each node in the tree.
	// Return false to stop walking children.
	Walk(vnode VNode, depth int) bool
}

// WalkFunc is a function type that implements Walker.
type WalkFunc func(VNode, int) bool

// Walk implements the Walker interface.
func (f WalkFunc) Walk(vnode VNode, depth int) bool {
	return f(vnode, depth)
}

// WalkTree traverses a VNode tree depth-first, calling the walker for each node.
func WalkTree(root VNode, walker Walker) {
	walkNode(root, walker, 0)
}

func walkNode(node VNode, walker Walker, depth int) {
	if !walker.Walk(node, depth) {
		return
	}
	for _, child := range node.Children {
		walkNode(child, walker, depth+1)
	}
}
