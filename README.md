# gox

JSX-like syntax for Go. Write components with familiar JSX patterns, get type-safe Go code.

```go
// button.gox
package ui

type ButtonProps struct {
    Label    string
    Disabled bool
}

func Button(props ButtonProps) runtime.VNode {
    return <button disabled={props.Disabled}>
        {props.Label}
    </button>
}
```

## Installation

### Homebrew (macOS/Linux)

```bash
brew tap germtb/tap
brew install gox
```

### Go install

```bash
go install github.com/germtb/gox/cmd/gox@latest
```

### Download binary

Download the latest release from [GitHub Releases](https://github.com/germtb/gox/releases).

```bash
# macOS (Apple Silicon)
curl -sSL https://github.com/germtb/gox/releases/latest/download/gox_darwin_arm64.tar.gz | tar xz
sudo mv gox /usr/local/bin/

# macOS (Intel)
curl -sSL https://github.com/germtb/gox/releases/latest/download/gox_darwin_amd64.tar.gz | tar xz
sudo mv gox /usr/local/bin/

# Linux (amd64)
curl -sSL https://github.com/germtb/gox/releases/latest/download/gox_linux_amd64.tar.gz | tar xz
sudo mv gox /usr/local/bin/
```

## Quick Start

1. Create a `.gox` file:

```go
// hello.gox
package main

import "github.com/germtb/gox/runtime"

func Hello() runtime.VNode {
    return <div>
        <h1>Hello, World!</h1>
    </div>
}
```

2. Run your project:

```bash
gox run .
```

That's it. Gox handles code generation automatically.

## Commands

| Command | Description |
|---------|-------------|
| `gox run [args]` | Generate and run with `go run` |
| `gox build [args]` | Generate and build with `go build` |
| `gox generate [path]` | Generate `.go` files from `.gox` files |
| `gox fmt [path]` | Format `.gox` files |
| `gox lsp` | Start LSP server (for IDE integration) |
| `gox version` | Print version |
| `gox help` | Show help |

### Examples

```bash
# Run a project
gox run ./cmd/myapp

# Build with output
gox build -o myapp ./cmd/myapp

# Generate all .gox files recursively
gox generate ./...

# Format and write changes
gox fmt -w ./...

# List files that need formatting
gox fmt -l .
```

## How It Works

Gox transforms `.gox` files into standard Go code:

```
button.gox  →  gox generate  →  button_gox.go
```

The generated code uses the `runtime.VNode` type and is fully type-checked by the Go compiler.

### Using gox vs go

**If your project has `.gox` files, use `gox` commands:**

```bash
gox run .      # instead of go run .
gox build .    # instead of go build .
```

**If your project is pure Go (even with gox dependencies), use standard Go:**

```bash
go run .       # works fine
go build .     # works fine
```

This is because gox libraries ship with pre-generated `.go` files.

## For Library Authors

If you're publishing a library that uses gox:

1. **Commit generated files** - Remove `*_gox.go` from `.gitignore`
2. **Ship both `.gox` and `*_gox.go`** - Consumers get working Go code

This allows consumers to `go get` your library without needing gox installed.

```
your-library/
├── component.gox           # Source (for contributors)
├── component_gox.go        # Generated (for consumers)
└── go.mod
```

### Why commit generated files?

| Approach | Pros | Cons |
|----------|------|------|
| Commit generated files | `go get` just works | Generated code in git |
| Gitignore generated files | Clean repo | Breaks `go get`, forces gox on consumers |

Most code generation tools (protobuf, sqlc, ent) recommend committing generated files for the same reason.

## For Application Developers

If you're building an application (not a library):

1. **Gitignore generated files** - Keep your repo clean
2. **Use `gox run` / `gox build`** - Handles generation automatically
3. **Add to CI** - Run `gox build` in your build pipeline

Example `.gitignore` for applications:

```gitignore
*_gox.go
*_gox.go.map
```

## Typed Props

Components with typed props get compile-time type checking:

```go
type ButtonProps struct {
    Label    string
    Disabled bool
    OnClick  func()
}

func Button(props ButtonProps) runtime.VNode {
    return <button disabled={props.Disabled} onClick={props.OnClick}>
        {props.Label}
    </button>
}

// Usage - props are type-checked!
func App() runtime.VNode {
    return <Button label="Submit" disabled={false} />
}
```

The Go compiler will catch:
- Missing required props
- Wrong prop types
- Typos in prop names

## VS Code Extension

Install the VS Code extension for:
- Syntax highlighting
- Format on save
- Go-to-definition (works across `.gox` and `.go` files)
- Error diagnostics

```bash
cd vscode-gox
npm install
npm run compile
# Then: "Developer: Install Extension from Location..."
```

Configure in `.vscode/settings.json`:

```json
{
  "files.associations": {
    "*.gox": "gox"
  },
  "[gox]": {
    "editor.formatOnSave": true
  }
}
```

## Source Maps

Gox generates source maps (`.map` files) that remap errors from generated code back to your `.gox` source:

```
button_gox.go:42:5: undefined: foo
         ↓ (remapped)
button.gox:15:5: undefined: foo
```

This works automatically with `gox run` and `gox build`.

## Project Structure

```
your-project/
├── cmd/
│   └── app/
│       └── main.gox          # Entry point with JSX
├── ui/
│   ├── button.gox            # Component
│   ├── button_gox.go         # Generated (gitignore for apps)
│   └── header.gox            # Component
├── go.mod
└── .gitignore
```

## FAQ

### Do consumers of my library need gox?

No. If you commit the generated `*_gox.go` files, consumers use standard `go get` and `go build`. They never need to know gox exists.

### Can I mix `.gox` and `.go` files?

Yes. They live side by side in the same package. `.go` files can import and use components defined in `.gox` files (via the generated code).

### What's the runtime overhead?

The runtime is minimal - just the `VNode` type and helper functions. There's no virtual DOM diffing or complex runtime. Generated code is straightforward Go.

### How do I render VNodes?

Gox generates a tree of `runtime.VNode` values. You provide a renderer for your target:

- **HTML**: Walk the tree and output HTML strings
- **Terminal**: Use a library like bubbletea or lipgloss
- **Testing**: Inspect the tree directly

See `demo/app.gox` for a terminal renderer example.

## License

MIT
