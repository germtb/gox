package gox

import (
	"testing"
)

func TestElement(t *testing.T) {
	node := Element("box", Props{"direction": "row"})

	if node.Type != "box" {
		t.Errorf("Element type = %v, want 'box'", node.Type)
	}
	if node.Props["direction"] != "row" {
		t.Errorf("Props[direction] = %v, want 'row'", node.Props["direction"])
	}
}

func TestElementWithChildren(t *testing.T) {
	child := Text("Hello")
	parent := Element("box", nil, child)

	if len(parent.Children) != 1 {
		t.Errorf("Children count = %d, want 1", len(parent.Children))
	}
	if !parent.Children[0].IsText() {
		t.Error("Child should be a text node")
	}
}

func TestText(t *testing.T) {
	node := Text("Hello, World!")

	if !node.IsText() {
		t.Error("Text node should return true for IsText()")
	}

	content, ok := node.GetTextContent()
	if !ok {
		t.Error("GetTextContent should return ok=true for text node")
	}
	if content != "Hello, World!" {
		t.Errorf("Text content = %q, want 'Hello, World!'", content)
	}
}

func TestFragment(t *testing.T) {
	child1 := Text("A")
	child2 := Text("B")
	frag := Fragment(child1, child2)

	if !frag.IsFragment() {
		t.Error("Fragment should return true for IsFragment()")
	}
	if len(frag.Children) != 2 {
		t.Errorf("Fragment children count = %d, want 2", len(frag.Children))
	}
}

func TestWhen(t *testing.T) {
	child := Text("Visible")

	result := When(true, child)
	if result.IsEmpty() {
		t.Error("When(true, ...) should return the child")
	}

	result = When(false, child)
	if !result.IsEmpty() {
		t.Error("When(false, ...) should return empty VNode")
	}
}

func TestWhenElse(t *testing.T) {
	ifTrue := Text("Yes")
	ifFalse := Text("No")

	result := WhenElse(true, ifTrue, ifFalse)
	content, _ := result.GetTextContent()
	if content != "Yes" {
		t.Errorf("WhenElse(true, ...) content = %q, want 'Yes'", content)
	}

	result = WhenElse(false, ifTrue, ifFalse)
	content, _ = result.GetTextContent()
	if content != "No" {
		t.Errorf("WhenElse(false, ...) content = %q, want 'No'", content)
	}
}

func TestMap(t *testing.T) {
	items := []string{"A", "B", "C"}
	nodes := Map(items, func(s string) VNode {
		return Text(s)
	})

	if len(nodes) != 3 {
		t.Errorf("Map result count = %d, want 3", len(nodes))
	}

	for i, node := range nodes {
		content, _ := node.GetTextContent()
		if content != items[i] {
			t.Errorf("Map result[%d] = %q, want %q", i, content, items[i])
		}
	}
}

func TestMapIndex(t *testing.T) {
	items := []string{"A", "B"}
	nodes := MapIndex(items, func(i int, s string) VNode {
		return Element("item", Props{"index": i, "value": s})
	})

	if len(nodes) != 2 {
		t.Errorf("MapIndex result count = %d, want 2", len(nodes))
	}
	if nodes[0].Props["index"] != 0 {
		t.Errorf("First item index = %v, want 0", nodes[0].Props["index"])
	}
}

func TestComponentElement(t *testing.T) {
	var MyComponent Component = func(props Props) VNode {
		return Element("div", props)
	}

	node := Element(MyComponent, Props{"id": "test"})

	if !node.IsComponent() {
		t.Error("Component element should return true for IsComponent()")
	}
}

func TestWalkTree(t *testing.T) {
	tree := Element("root", nil,
		Element("child1", nil,
			Text("leaf1"),
		),
		Element("child2", nil,
			Text("leaf2"),
		),
	)

	var visited []string
	WalkTree(tree, WalkFunc(func(node VNode, depth int) bool {
		if s, ok := node.Type.(string); ok {
			visited = append(visited, s)
		}
		return true
	}))

	expected := []string{"root", "child1", TextNodeType, "child2", TextNodeType}
	if len(visited) != len(expected) {
		t.Errorf("Visited count = %d, want %d", len(visited), len(expected))
	}
	for i, v := range visited {
		if v != expected[i] {
			t.Errorf("visited[%d] = %q, want %q", i, v, expected[i])
		}
	}
}

func TestEmpty(t *testing.T) {
	empty := Empty()
	if !empty.IsEmpty() {
		t.Error("Empty() should return an empty VNode")
	}
}
