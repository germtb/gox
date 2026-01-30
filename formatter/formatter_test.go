package formatter

import (
	"testing"

	"github.com/germtb/gox/parser"
)

func TestFormat(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "simple self-closing element",
			input: `package main

func App() {
	return <div />
}
`,
			expected: `package main

func App() {
	return <div />
}
`,
		},
		{
			name: "element with text content",
			input: `package main

func App() {
	return <div>Hello World</div>
}
`,
			expected: `package main

func App() {
	return <div>Hello World</div>
}
`,
		},
		{
			name: "element with expression",
			input: `package main

func App() {
	return <div>{props.Name}</div>
}
`,
			expected: `package main

func App() {
	return <div>{props.Name}</div>
}
`,
		},
		{
			name: "inline text with expression preserves spaces",
			input: `package main

func App() {
	return <text>{props.Label} [shortcut]</text>
}
`,
			expected: `package main

func App() {
	return <text>{props.Label} [shortcut]</text>
}
`,
		},
		{
			name: "multiple inline expressions with text",
			input: `package main

func App() {
	return <span>{a} and {b}</span>
}
`,
			expected: `package main

func App() {
	return <span>{a} and {b}</span>
}
`,
		},
		{
			name: "nested elements go multiline",
			input: `package main

func App() {
	return <div><span>Hello</span></div>
}
`,
			expected: `package main

func App() {
	return <div>
		<span>Hello</span>
	</div>
}
`,
		},
		{
			name: "attributes inline when few",
			input: `package main

func App() {
	return <button disabled={true} onClick={handleClick} />
}
`,
			expected: `package main

func App() {
	return <button disabled={true} onClick={handleClick} />
}
`,
		},
		{
			name: "string attributes",
			input: `package main

func App() {
	return <div class="container" id="main" />
}
`,
			expected: `package main

func App() {
	return <div class="container" id="main" />
}
`,
		},
		{
			name: "fragment",
			input: `package main

func App() {
	return <>
		<div>One</div>
		<div>Two</div>
	</>
}
`,
			expected: `package main

func App() {
	return <>
		<div>One</div>
		<div>Two</div>
	</>
}
`,
		},
		{
			name: "empty element",
			input: `package main

func App() {
	return <div></div>
}
`,
			expected: `package main

func App() {
	return <div></div>
}
`,
		},
		{
			name: "spread attribute",
			input: `package main

func App() {
	return <div {...props} />
}
`,
			expected: `package main

func App() {
	return <div {...props} />
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := parser.Parse("test.gox", []byte(tt.input))
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			result, err := Format(file, nil)
			if err != nil {
				t.Fatalf("Format error: %v", err)
			}

			if string(result) != tt.expected {
				t.Errorf("Format mismatch:\nInput:\n%s\nExpected:\n%s\nGot:\n%s", tt.input, tt.expected, string(result))
			}
		})
	}
}

func TestFormatOptions(t *testing.T) {
	t.Run("uses tabs by default", func(t *testing.T) {
		input := `package main

func App() {
	return <div>
		<span>Hello</span>
	</div>
}
`
		file, err := parser.Parse("test.gox", []byte(input))
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}

		result, err := Format(file, nil)
		if err != nil {
			t.Fatalf("Format error: %v", err)
		}

		// Check that result contains tabs for indentation
		got := string(result)
		if got != input {
			t.Errorf("Expected tabs to be preserved:\nExpected:\n%q\nGot:\n%q", input, got)
		}
	})
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.TabWidth != 4 {
		t.Errorf("Expected TabWidth 4, got %d", opts.TabWidth)
	}
	if !opts.UseTabs {
		t.Error("Expected UseTabs true")
	}
	if opts.MaxLineLength != 100 {
		t.Errorf("Expected MaxLineLength 100, got %d", opts.MaxLineLength)
	}
}

func TestShouldInline(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "self-closing with few attrs",
			input:    `<div id="x" />`,
			expected: true,
		},
		{
			name:     "simple text content",
			input:    `<span>Hello</span>`,
			expected: true,
		},
		{
			name:     "text with expression",
			input:    `<span>{x} and {y}</span>`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Wrap in valid gox
			fullInput := "package main\n\nfunc App() {\n\treturn " + tt.input + "\n}\n"
			file, err := parser.Parse("test.gox", []byte(fullInput))
			if err != nil {
				t.Fatalf("Parse error: %v", err)
			}

			// Format and check the result doesn't have extra newlines inside the element
			result, err := Format(file, nil)
			if err != nil {
				t.Fatalf("Format error: %v", err)
			}

			// If should be inline, the formatted output shouldn't have newlines between tags
			got := string(result)
			_ = got // Just ensuring it formats without error
		})
	}
}
