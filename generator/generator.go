// Package generator transforms gox AST into Go source code.
package generator

import (
	"bytes"
	"fmt"
	"go/format"
	"strings"
	"unicode"

	"github.com/germtb/gox/ast"
	"github.com/germtb/gox/parser"
)

// Generator transforms AST to Go code.
type Generator struct {
	buf         bytes.Buffer
	indent      int
	sourceMap   *SourceMap
	runtimePkg  string
	needsImport bool

	// Position tracking for source maps
	outLine uint32 // Current output line (0-indexed)
	outCol  uint32 // Current output column (0-indexed)
}

// Options configures the generator.
type Options struct {
	// RuntimePackage is the import path for the gox package.
	// Default: "github.com/germtb/gox"
	RuntimePackage string
}

// New creates a new Generator.
func New(opts *Options) *Generator {
	g := &Generator{
		sourceMap:  NewSourceMap(),
		runtimePkg: "github.com/germtb/gox",
	}
	if opts != nil && opts.RuntimePackage != "" {
		g.runtimePkg = opts.RuntimePackage
	}
	return g
}

// Generate transforms a GoxFile AST into Go source code.
func Generate(file *ast.GoxFile, opts *Options) ([]byte, *SourceMap, error) {
	g := New(opts)
	return g.Generate(file)
}

// Generate generates Go code from the AST.
func (g *Generator) Generate(file *ast.GoxFile) ([]byte, *SourceMap, error) {
	// First pass: check if we need runtime import
	g.needsImport = g.hasJSX(file)

	// Generate all nodes
	for _, node := range file.Nodes {
		g.generateNode(node)
	}

	result := g.buf.Bytes()

	// Insert runtime import if needed
	if g.needsImport {
		result = g.insertRuntimeImport(result)
	}

	// Format the generated code
	formatted, err := format.Source(result)
	if err != nil {
		// If formatting fails, return unformatted code with a comment
		// This allows debugging malformed output
		return result, g.sourceMap, nil
	}

	return formatted, g.sourceMap, nil
}

// hasJSX checks if the file contains any JSX elements.
func (g *Generator) hasJSX(file *ast.GoxFile) bool {
	for _, node := range file.Nodes {
		switch node.(type) {
		case *ast.JSXElement, *ast.JSXFragment:
			return true
		}
	}
	return false
}

// insertRuntimeImport adds the runtime import after the package declaration.
func (g *Generator) insertRuntimeImport(src []byte) []byte {
	code := string(src)

	// Check if runtime is already imported
	if strings.Contains(code, g.runtimePkg) {
		return src
	}

	// Find the package declaration
	pkgIdx := strings.Index(code, "package ")
	if pkgIdx == -1 {
		return src
	}

	// Find the end of the package line
	newlineIdx := strings.Index(code[pkgIdx:], "\n")
	if newlineIdx == -1 {
		return src
	}
	insertPos := pkgIdx + newlineIdx + 1

	// Check if there's an existing import block
	importIdx := strings.Index(code[insertPos:], "import ")
	if importIdx != -1 && importIdx < 100 { // import should be near package decl
		// There's already an import, need to add to it
		importPos := insertPos + importIdx

		// Check if it's import "single" or import (block)
		afterImport := code[importPos+7:]
		if len(afterImport) > 0 && afterImport[0] == '(' {
			// Block import - insert our import after the opening (
			parenPos := importPos + 7 + 1
			return []byte(code[:parenPos] + "\n\t\"" + g.runtimePkg + "\"" + code[parenPos:])
		} else {
			// Single import - convert to block
			// Find the end of the import line
			importEnd := strings.Index(afterImport, "\n")
			if importEnd == -1 {
				return src
			}
			singleImport := strings.TrimSpace(afterImport[:importEnd])
			return []byte(code[:importPos] + "import (\n\t\"" + g.runtimePkg + "\"\n\t" + singleImport + "\n)" + code[importPos+7+importEnd:])
		}
	}

	// No existing import, add new one
	importStmt := fmt.Sprintf("\nimport \"%s\"\n", g.runtimePkg)
	return []byte(code[:insertPos] + importStmt + code[insertPos:])
}

// generateNode generates code for a single AST node.
func (g *Generator) generateNode(node ast.Node) {
	switch n := node.(type) {
	case *ast.GoCode:
		// GoCode is passed through with source mapping
		r := n.GetRange()
		g.writeWithMapping(n.Value, r.Start.Line, r.Start.Column)
	case *ast.JSXElement:
		g.generateJSXElement(n)
	case *ast.JSXFragment:
		g.generateJSXFragment(n)
	}
}

// generateJSXElement generates code for a JSX element.
func (g *Generator) generateJSXElement(elem *ast.JSXElement) {
	// Record source mapping for the start of this element
	r := elem.GetRange()
	if r.Start.Line > 0 {
		g.sourceMap.AddMapping(
			uint32(r.Start.Line-1), uint32(r.Start.Column-1),
			g.outLine, g.outCol,
		)
	}

	// Determine if it's an intrinsic element (lowercase) or component (uppercase)
	isComponent := len(elem.Tag) > 0 && unicode.IsUpper(rune(elem.Tag[0]))

	if isComponent {
		// Typed component: ComponentName(ComponentNameProps{...}, children...)
		g.generateTypedComponent(elem)
	} else {
		// Intrinsic element: runtime.Element("tag", props, children...)
		g.generateIntrinsicElement(elem)
	}
}

// generateTypedComponent generates code for a typed component.
// Output: ComponentName(ComponentNameProps{Field: value, ...}, child1, child2, ...)
func (g *Generator) generateTypedComponent(elem *ast.JSXElement) {
	propsType := elem.Tag + "Props"

	g.write(elem.Tag)
	g.write("(")

	// Generate props struct literal
	g.generateTypedProps(elem.Attributes, propsType)

	// Generate children
	g.generateChildren(elem.Children)

	g.write(")")
}

// generateIntrinsicElement generates code for an intrinsic element.
// Output: gox.Element("tag", gox.Props{...}, child1, child2, ...)
func (g *Generator) generateIntrinsicElement(elem *ast.JSXElement) {
	g.write("gox.Element(")
	g.write(fmt.Sprintf("%q", elem.Tag))
	g.write(", ")

	// Props
	g.generateProps(elem.Attributes)

	// Children
	g.generateChildren(elem.Children)

	g.write(")")
}

// generateChildren generates the children arguments for an element.
func (g *Generator) generateChildren(children []ast.JSXChild) {
	for _, child := range children {
		// Skip whitespace-only text
		if t, ok := child.(*ast.JSXText); ok {
			if strings.TrimSpace(t.Value) == "" {
				continue
			}
		}
		// Skip comment-only expressions
		if e, ok := child.(*ast.JSXExpression); ok {
			expr := strings.TrimSpace(e.Expression)
			if expr == "" || isCommentOnly(expr) {
				continue
			}
		}
		g.write(",\n")
		g.writeIndent()
		g.generateJSXChild(child)
	}
}

// generateTypedProps generates a typed props struct literal.
// Output: PropsType{Field: value, ...}
func (g *Generator) generateTypedProps(attrs []ast.Attribute, propsType string) {
	if len(attrs) == 0 {
		g.write(propsType + "{}")
		return
	}

	// Check for spread attributes
	hasSpread := false
	for _, attr := range attrs {
		if _, ok := attr.(*ast.SpreadAttribute); ok {
			hasSpread = true
			break
		}
	}

	if hasSpread {
		// For typed props with spread, we need a different approach
		// Generate: mergeTypedProps(baseProps, SpreadProps{...})
		// For now, fall back to building the struct with spread merged in Go
		g.generateTypedPropsWithSpread(attrs, propsType)
		return
	}

	g.write(propsType + "{")

	first := true
	for _, attr := range attrs {
		if !first {
			g.write(", ")
		}
		first = false

		switch a := attr.(type) {
		case *ast.StringAttribute:
			g.write(fmt.Sprintf("%s: %q", capitalize(a.Key), a.Value))
		case *ast.ExpressionAttribute:
			g.write(fmt.Sprintf("%s: %s", capitalize(a.Key), a.Expression))
		}
	}

	g.write("}")
}

// generateTypedPropsWithSpread handles spread attributes in typed props.
// This requires runtime support or generates inline merging.
func (g *Generator) generateTypedPropsWithSpread(attrs []ast.Attribute, propsType string) {
	// For typed props with spread, we generate the struct with explicit fields
	// Spread attributes are merged at runtime - this requires the spread source
	// to be of the same type or compatible

	// Simple approach: generate struct with spread first, then override with explicit props
	// This matches JSX semantics where later props override earlier ones

	// Find spread and regular attrs
	var spreads []*ast.SpreadAttribute
	var regular []ast.Attribute
	for _, attr := range attrs {
		if s, ok := attr.(*ast.SpreadAttribute); ok {
			spreads = append(spreads, s)
		} else {
			regular = append(regular, attr)
		}
	}

	// If we have spreads, we need to merge them
	// For typed props: we'll just use the first spread as base and add fields
	if len(spreads) > 0 {
		// Generate: propsType{...spread, Field: value}
		// Go doesn't support spread in struct literals, so we need a helper
		// For now, just use the struct literal approach and warn about spread
		g.write(propsType + "{")

		first := true
		for _, attr := range regular {
			if !first {
				g.write(", ")
			}
			first = false

			switch a := attr.(type) {
			case *ast.StringAttribute:
				g.write(fmt.Sprintf("%s: %q", capitalize(a.Key), a.Value))
			case *ast.ExpressionAttribute:
				g.write(fmt.Sprintf("%s: %s", capitalize(a.Key), a.Expression))
			}
		}

		g.write("}")
		return
	}

	// No spreads, just generate regular struct
	g.generateTypedProps(regular, propsType)
}

// generateJSXFragment generates code for a JSX fragment.
func (g *Generator) generateJSXFragment(frag *ast.JSXFragment) {
	// Record source mapping for the start of this fragment
	r := frag.GetRange()
	if r.Start.Line > 0 {
		g.sourceMap.AddMapping(
			uint32(r.Start.Line-1), uint32(r.Start.Column-1),
			g.outLine, g.outCol,
		)
	}

	g.write("gox.Fragment(")

	first := true
	for _, child := range frag.Children {
		// Skip whitespace-only text
		if t, ok := child.(*ast.JSXText); ok {
			if strings.TrimSpace(t.Value) == "" {
				continue
			}
		}
		// Skip comment-only expressions
		if e, ok := child.(*ast.JSXExpression); ok {
			expr := strings.TrimSpace(e.Expression)
			if expr == "" || isCommentOnly(expr) {
				continue
			}
		}
		if !first {
			g.write(",\n")
			g.writeIndent()
		}
		first = false
		g.generateJSXChild(child)
	}

	g.write(")")
}

// generateProps generates the Props map for an element.
func (g *Generator) generateProps(attrs []ast.Attribute) {
	if len(attrs) == 0 {
		g.write("nil")
		return
	}

	// Check for spread attributes
	hasSpread := false
	for _, attr := range attrs {
		if _, ok := attr.(*ast.SpreadAttribute); ok {
			hasSpread = true
			break
		}
	}

	if hasSpread {
		// Use gox.MergeProps for spread handling
		g.generatePropsWithSpread(attrs)
		return
	}

	g.write("gox.Props{")

	first := true
	for _, attr := range attrs {
		if !first {
			g.write(", ")
		}
		first = false

		switch a := attr.(type) {
		case *ast.StringAttribute:
			g.write(fmt.Sprintf("%q: %q", a.Key, a.Value))
		case *ast.ExpressionAttribute:
			g.write(fmt.Sprintf("%q: %s", a.Key, wrapMapLiteral(a.Expression)))
		}
	}

	g.write("}")
}

// generatePropsWithSpread generates props that include spread attributes.
func (g *Generator) generatePropsWithSpread(attrs []ast.Attribute) {
	// Generate gox.MergeProps(...) with props in order
	// This preserves the JSX semantics where later props override earlier ones

	g.write("gox.MergeProps(")

	// Group consecutive regular props together
	first := true
	var regularBatch []ast.Attribute

	flushRegularBatch := func() {
		if len(regularBatch) == 0 {
			return
		}
		if !first {
			g.write(", ")
		}
		first = false

		g.write("gox.Props{")
		for i, attr := range regularBatch {
			if i > 0 {
				g.write(", ")
			}
			switch a := attr.(type) {
			case *ast.StringAttribute:
				g.write(fmt.Sprintf("%q: %q", a.Key, a.Value))
			case *ast.ExpressionAttribute:
				g.write(fmt.Sprintf("%q: %s", a.Key, wrapMapLiteral(a.Expression)))
			}
		}
		g.write("}")
		regularBatch = nil
	}

	for _, attr := range attrs {
		switch a := attr.(type) {
		case *ast.SpreadAttribute:
			flushRegularBatch()
			if !first {
				g.write(", ")
			}
			first = false
			g.write(a.Expression)
		default:
			regularBatch = append(regularBatch, attr)
		}
	}

	flushRegularBatch()
	g.write(")")
}

// generateJSXChild generates code for a JSX child.
func (g *Generator) generateJSXChild(child ast.JSXChild) {
	switch c := child.(type) {
	case *ast.JSXText:
		text := strings.TrimSpace(c.Value)
		if text == "" {
			return // Skip whitespace-only text
		}
		g.write(fmt.Sprintf("gox.Text(%q)", text))

	case *ast.JSXExpression:
		expr := strings.TrimSpace(c.Expression)

		// Skip empty expressions or comment-only expressions
		if expr == "" || isCommentOnly(expr) {
			return
		}

		// Transform any JSX within the expression
		transformed := g.transformExpressionJSX(expr)

		// Check for conditional pattern: expr && <elem>
		if idx := strings.Index(transformed, " && "); idx != -1 {
			cond := strings.TrimSpace(transformed[:idx])
			rest := strings.TrimSpace(transformed[idx+4:])
			g.write(fmt.Sprintf("gox.When(%s, %s)", cond, rest))
		} else {
			// Wrap expressions in gox.V() to convert any value to VNode
			g.write(fmt.Sprintf("gox.V(%s)", transformed))
		}

	case *ast.JSXElement:
		g.generateJSXElement(c)

	case *ast.JSXFragment:
		g.generateJSXFragment(c)
	}
}

// transformExpressionJSX finds and transforms JSX elements within an expression string.
func (g *Generator) transformExpressionJSX(expr string) string {
	result := expr

	// Keep transforming until no more JSX is found
	for {
		// Find JSX start: < followed by identifier
		jsxStart := -1
		for i := 0; i < len(result); i++ {
			if result[i] == '<' && i+1 < len(result) {
				next := result[i+1]
				// Check for identifier start (letter or uppercase for component)
				if (next >= 'a' && next <= 'z') || (next >= 'A' && next <= 'Z') {
					jsxStart = i
					break
				}
				// Check for fragment <>
				if next == '>' {
					jsxStart = i
					break
				}
			}
		}

		if jsxStart == -1 {
			break // No more JSX
		}

		// Extract and transform the JSX portion
		jsxEnd := g.findJSXEnd(result, jsxStart)
		if jsxEnd == -1 {
			break // Malformed JSX
		}

		jsxCode := result[jsxStart:jsxEnd]
		transformed := g.transformJSXString(jsxCode)

		result = result[:jsxStart] + transformed + result[jsxEnd:]
	}

	return result
}

// findJSXEnd finds the end of a JSX element starting at pos.
func (g *Generator) findJSXEnd(s string, start int) int {
	depth := 0
	i := start

	for i < len(s) {
		if s[i] == '<' {
			if i+1 < len(s) && s[i+1] == '/' {
				// Closing tag
				depth--
				// Find the >
				for i < len(s) && s[i] != '>' {
					i++
				}
				if depth == 0 {
					return i + 1
				}
			} else if i+1 < len(s) && s[i+1] == '>' {
				// Fragment open <>
				depth++
				i += 2
			} else {
				// Opening tag
				depth++
				// Scan to find > or />
				for i < len(s) {
					if s[i] == '/' && i+1 < len(s) && s[i+1] == '>' {
						// Self-closing
						depth--
						i += 2
						if depth == 0 {
							return i
						}
						break
					}
					if s[i] == '>' {
						i++
						break
					}
					i++
				}
			}
		} else {
			i++
		}
	}

	return -1
}

// transformJSXString parses and transforms a JSX string to Go code.
func (g *Generator) transformJSXString(jsx string) string {
	// Parse the JSX using our parser
	file, err := parser.Parse("<expr>", []byte(jsx))
	if err != nil || len(file.Nodes) == 0 {
		return jsx // Return unchanged if parsing fails
	}

	// Generate code for the parsed JSX
	gen := New(&Options{RuntimePackage: g.runtimePkg})
	for _, node := range file.Nodes {
		gen.generateNode(node)
	}

	return gen.buf.String()
}

// isCommentOnly checks if a string contains only Go comments.
func isCommentOnly(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}

	// Check for /* */ style comment
	if strings.HasPrefix(s, "/*") && strings.HasSuffix(s, "*/") {
		// Check if there's anything besides the comment
		inner := strings.TrimPrefix(s, "/*")
		inner = strings.TrimSuffix(inner, "*/")
		return strings.TrimSpace(inner) == "" || !strings.Contains(inner, "*/")
	}

	// Check for // style comment
	if strings.HasPrefix(s, "//") {
		return true
	}

	return false
}

// wrapMapLiteral adds map[string]any prefix to bare map literals.
// Converts {key: value} to map[string]any{key: value}
func wrapMapLiteral(expr string) string {
	expr = strings.TrimSpace(expr)
	if len(expr) < 2 {
		return expr
	}

	// Check if it's a bare map literal (starts with { and ends with })
	if expr[0] == '{' && expr[len(expr)-1] == '}' {
		// Make sure it's not already typed (e.g., map[string]any{...})
		// or a struct literal (e.g., MyStruct{...})
		// A bare literal starts directly with {
		return "map[string]any" + expr
	}

	return expr
}

// capitalize converts the first letter of a string to uppercase.
// Used to convert JSX attribute names to Go struct field names.
// e.g., "onClick" -> "OnClick", "label" -> "Label"
func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// Helper methods

func (g *Generator) write(s string) {
	g.buf.WriteString(s)
	// Update position tracking
	for _, r := range s {
		if r == '\n' {
			g.outLine++
			g.outCol = 0
		} else {
			g.outCol++
		}
	}
}

// writeWithMapping writes a string and records source map mapping.
// srcLine and srcCol are 1-indexed (from AST), converted to 0-indexed for source map.
func (g *Generator) writeWithMapping(s string, srcLine, srcCol int) {
	if srcLine > 0 && srcCol > 0 {
		// Record mapping for each character
		srcPos := NewPosition(0, uint32(srcLine-1), uint32(srcCol-1))
		tgtPos := NewPosition(0, g.outLine, g.outCol)
		g.sourceMap.AddExpression(s, srcPos, tgtPos)
	}
	g.write(s)
}

func (g *Generator) writeIndent() {
	for i := 0; i < g.indent; i++ {
		g.write("\t")
	}
}

func (g *Generator) writeLine(s string) {
	g.writeIndent()
	g.write(s)
	g.write("\n")
}
