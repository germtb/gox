package generator

import (
	"strings"
	"testing"

	"github.com/germtb/gox/parser"
)

func TestGenerateSimpleElement(t *testing.T) {
	src := `package main

func App() {
	return <box></box>
}`

	file, err := parser.Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	output, _, err := Generate(file, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	code := string(output)

	// Check that runtime import was added
	if !strings.Contains(code, `import "github.com/germtb/gox/runtime"`) {
		t.Errorf("Expected runtime import, got:\n%s", code)
	}

	// Check that Element() call was generated
	if !strings.Contains(code, `runtime.Element("box", nil)`) {
		t.Errorf("Expected Element call, got:\n%s", code)
	}
}

func TestGenerateSelfClosingElement(t *testing.T) {
	src := `<input />`

	file, err := parser.Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	output, _, err := Generate(file, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	code := string(output)

	if !strings.Contains(code, `runtime.Element("input", nil)`) {
		t.Errorf("Expected Element call, got:\n%s", code)
	}
}

func TestGenerateElementWithAttributes(t *testing.T) {
	src := `<box direction="row" gap={1}></box>`

	file, err := parser.Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	output, _, err := Generate(file, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	code := string(output)

	// Check for props
	if !strings.Contains(code, `"direction": "row"`) {
		t.Errorf("Expected direction prop, got:\n%s", code)
	}
	if !strings.Contains(code, `"gap": 1`) {
		t.Errorf("Expected gap prop, got:\n%s", code)
	}
}

func TestGenerateElementWithTextChild(t *testing.T) {
	src := `<text>Hello World</text>`

	file, err := parser.Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	output, _, err := Generate(file, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	code := string(output)

	if !strings.Contains(code, `runtime.Text("Hello World")`) {
		t.Errorf("Expected Text call, got:\n%s", code)
	}
}

func TestGenerateElementWithExpressionChild(t *testing.T) {
	src := `<text>{name}</text>`

	file, err := parser.Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	output, _, err := Generate(file, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	code := string(output)

	// Expression should be passed through directly
	if !strings.Contains(code, "name") {
		t.Errorf("Expected name expression, got:\n%s", code)
	}
}

func TestGenerateNestedElements(t *testing.T) {
	src := `<box><text>Hi</text></box>`

	file, err := parser.Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	output, _, err := Generate(file, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	code := string(output)

	// Should have nested Element calls
	if !strings.Contains(code, `runtime.Element("box"`) {
		t.Errorf("Expected box element, got:\n%s", code)
	}
	if !strings.Contains(code, `runtime.Element("text"`) {
		t.Errorf("Expected text element, got:\n%s", code)
	}
}

func TestGenerateFragment(t *testing.T) {
	src := `<>Hello</>`

	file, err := parser.Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	output, _, err := Generate(file, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	code := string(output)

	if !strings.Contains(code, "runtime.Fragment(") {
		t.Errorf("Expected Fragment call, got:\n%s", code)
	}
}

func TestGenerateComponentElement(t *testing.T) {
	src := `<MyComponent foo="bar" />`

	file, err := parser.Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	output, _, err := Generate(file, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	code := string(output)

	// Component should generate typed props call
	if !strings.Contains(code, "MyComponent(MyComponentProps{Foo: \"bar\"})") {
		t.Errorf("Expected typed component call, got:\n%s", code)
	}
	// Should NOT use runtime.Element for components
	if strings.Contains(code, "runtime.Element") && strings.Contains(code, "MyComponent") {
		t.Errorf("Should not use runtime.Element for typed components, got:\n%s", code)
	}
}

func TestGenerateComponentWithChildren(t *testing.T) {
	src := `<Button label="Click">Hello</Button>`

	file, err := parser.Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	output, _, err := Generate(file, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	code := string(output)

	// Component with children: Button(ButtonProps{Label: "Click"}, runtime.Text("Hello"))
	if !strings.Contains(code, "Button(ButtonProps{Label: \"Click\"}") {
		t.Errorf("Expected typed component call with props, got:\n%s", code)
	}
	if !strings.Contains(code, "runtime.Text(\"Hello\")") {
		t.Errorf("Expected text child, got:\n%s", code)
	}
}

func TestGenerateComponentWithExpressionProp(t *testing.T) {
	src := `<Toggle enabled={true} onChange={handleChange} />`

	file, err := parser.Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	output, _, err := Generate(file, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	code := string(output)

	// Expression props should be passed directly (not quoted)
	if !strings.Contains(code, "Toggle(ToggleProps{Enabled: true, OnChange: handleChange})") {
		t.Errorf("Expected expression props, got:\n%s", code)
	}
}

func TestGeneratePreservesGoCode(t *testing.T) {
	src := `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}
`

	file, err := parser.Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	output, _, err := Generate(file, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	code := string(output)

	// Should preserve Go code
	if !strings.Contains(code, "package main") {
		t.Errorf("Expected package declaration, got:\n%s", code)
	}
	if !strings.Contains(code, `fmt.Println("Hello")`) {
		t.Errorf("Expected fmt.Println, got:\n%s", code)
	}

	// Should NOT add runtime import (no JSX)
	if strings.Contains(code, "gox/runtime") {
		t.Errorf("Should not add runtime import when no JSX, got:\n%s", code)
	}
}

func TestGenerateComplexExample(t *testing.T) {
	src := `package main

func App(props AppProps) runtime.VNode {
	return <box direction="row" gap={1}>
		<text style={{color: "blue"}}>Hello</text>
	</box>
}`

	file, err := parser.Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	output, _, err := Generate(file, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	code := string(output)

	// Check basic structure
	if !strings.Contains(code, "package main") {
		t.Errorf("Expected package declaration")
	}
	if !strings.Contains(code, "runtime.Element") {
		t.Errorf("Expected Element call")
	}
}

func TestGenerateCustomRuntimePackage(t *testing.T) {
	src := `package main

func App() {
	return <box></box>
}`

	file, err := parser.Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	output, _, err := Generate(file, &Options{
		RuntimePackage: "myapp/ui",
	})
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	code := string(output)

	if !strings.Contains(code, `import "myapp/ui"`) {
		t.Errorf("Expected custom import, got:\n%s", code)
	}
}

func TestGenerateSourceMapPopulated(t *testing.T) {
	src := `package main

func App() {
	return <box></box>
}`

	file, err := parser.Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	_, sm, err := Generate(file, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	if !sm.HasMappings() {
		t.Error("Expected source map to have mappings after generation")
	}

	// Check that we can find some source positions from target
	// The generated code should have mappings
	found := false
	for line := uint32(0); line < 20; line++ {
		for col := uint32(0); col < 50; col++ {
			if _, ok := sm.SourcePositionFromTarget(line, col); ok {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		t.Error("Expected to find at least one source position from target")
	}
}
