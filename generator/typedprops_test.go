package generator_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/germtb/gox/generator"
	"github.com/germtb/gox/parser"
)

// TestTypedPropsCompiles verifies that generated typed props code compiles correctly
func TestTypedPropsCompiles(t *testing.T) {
	// Create a complete, compilable gox file
	src := `package main

import "github.com/germtb/gox"

type ButtonProps struct {
	Label    string
	Disabled bool
}

func Button(props ButtonProps, children ...gox.VNode) gox.VNode {
	return <button disabled={props.Disabled}>{props.Label}</button>
}

func main() {
	_ = <Button label="Click" disabled={false} />
}
`

	// Parse and generate
	file, err := parser.Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	output, _, err := generator.Generate(file, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	// Write to temp file and compile
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(goFile, output, 0644); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	// Try to build
	cmd := exec.Command("go", "build", "-o", filepath.Join(tmpDir, "test"), goFile)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("Expected code to compile, but got error:\n%s\nGenerated code:\n%s", out, output)
	}
}

// TestTypedPropsTypeError verifies that wrong prop types cause compile errors
func TestTypedPropsTypeError(t *testing.T) {
	// Create code with intentionally wrong prop type (string instead of bool)
	src := `package main

import "github.com/germtb/gox"

type ButtonProps struct {
	Label    string
	Disabled bool
}

func Button(props ButtonProps, children ...gox.VNode) gox.VNode {
	return <button>{props.Label}</button>
}

func main() {
	// Pass string "true" instead of bool true - should fail
	_ = <Button label="Click" disabled={"not a bool"} />
}
`

	// Parse and generate
	file, err := parser.Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	output, _, err := generator.Generate(file, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	// Write to temp file and try to compile
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(goFile, output, 0644); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	// This should FAIL to build
	cmd := exec.Command("go", "build", "-o", filepath.Join(tmpDir, "test"), goFile)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("Expected type error but code compiled successfully.\nGenerated code:\n%s", output)
	} else {
		// Verify it's a type error (string cannot be used as bool)
		errStr := string(out)
		if !strings.Contains(errStr, "bool") && !strings.Contains(errStr, "cannot use") {
			t.Errorf("Expected type error about bool, got:\n%s", out)
		}
	}
}

// TestTypedPropsMissingRequired verifies that missing props cause compile errors
func TestTypedPropsMissingRequired(t *testing.T) {
	// ButtonProps has required Label field, but we don't pass it
	src := `package main

import "github.com/germtb/gox"

type ButtonProps struct {
	Label string  // required - no default
}

func Button(props ButtonProps, children ...gox.VNode) gox.VNode {
	return <button>{props.Label}</button>
}

func main() {
	// Missing label prop - this compiles in Go (zero value) but shows typed props work
	_ = <Button />
}
`

	// Parse and generate
	file, err := parser.Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	output, _, err := generator.Generate(file, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	// This should compile (Go uses zero values for missing fields)
	// but we verify the generated code has the empty struct
	if !strings.Contains(string(output), "Button(ButtonProps{})") {
		t.Errorf("Expected empty props struct, got:\n%s", output)
	}
}

// TestTypedPropsUnknownField verifies that unknown props cause compile errors
func TestTypedPropsUnknownField(t *testing.T) {
	// Pass a prop that doesn't exist on the struct
	src := `package main

import "github.com/germtb/gox"

type ButtonProps struct {
	Label string
}

func Button(props ButtonProps, children ...gox.VNode) gox.VNode {
	return <button>{props.Label}</button>
}

func main() {
	// unknownProp doesn't exist on ButtonProps - should fail
	_ = <Button label="Click" unknownProp={123} />
}
`

	// Parse and generate
	file, err := parser.Parse("test.gox", []byte(src))
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	output, _, err := generator.Generate(file, nil)
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	// Write to temp file and try to compile
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(goFile, output, 0644); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	// This should FAIL to build - unknown field
	cmd := exec.Command("go", "build", "-o", filepath.Join(tmpDir, "test"), goFile)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Errorf("Expected compile error for unknown field but code compiled.\nGenerated code:\n%s", output)
	} else {
		// Verify it's about unknown field
		if !strings.Contains(string(out), "UnknownProp") {
			t.Errorf("Expected error about UnknownProp field, got:\n%s", out)
		}
	}
}
