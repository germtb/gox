# Claude Instructions for gox

## Project Overview

gox is a JSX-like syntax preprocessor for Go. It transforms `.gox` files containing Go code with JSX syntax into pure Go code.

## Key Commands

```bash
# Build the gox binary
go build -o /tmp/gox ./cmd/gox

# Run tests
/tmp/gox test ./...

# Format .gox files
/tmp/gox fmt -w ./...

# Generate Go code from .gox files
/tmp/gox generate ./...

# Run a gox project
/tmp/gox run ./demo/
```

## Project Structure

- `cmd/gox/` - CLI entry point
- `lexer/` - Tokenizer for .gox syntax
- `parser/` - Parser that produces AST
- `generator/` - Transforms AST to Go code with source maps
- `formatter/` - Formats .gox files
- `lsp/` - LSP server (proxies to gopls)
- `vscode-gox/` - VS Code extension
- `ast/` - AST node types
- Root package (`gox`) - VNode, Props, and helper functions

## Code Generation

Generated files use the `_gox.go` suffix (e.g., `app.gox` → `app_gox.go`). Test files use `_gox_test.go` (e.g., `features_test.gox` → `features_gox_test.go`).

The generator produces code like:
```go
gox.Element("div", gox.Props{"class": "container"},
    gox.Element("text", nil,
        gox.Text("Hello")))
```

## Testing

Always run tests after making changes:
```bash
/tmp/gox test ./...
```

For quick iteration, build to /tmp:
```bash
go build -o /tmp/gox ./cmd/gox
```

## LSP Notes

The LSP proxies to gopls for Go intelligence. Key behaviors:
- Formatting is handled directly by gox (not gopls)
- Code actions return empty for .gox files
- Source maps translate positions between .gox and generated .go

## Generated Files

Generated files should NOT be committed. They are:
- Created in temp directories during `gox run/build` (overlay mode)
- Gitignored via `*_gox.go` and `*_gox_test.go` patterns
