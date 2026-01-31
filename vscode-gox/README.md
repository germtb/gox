# Gox Language Support

Language support for `.gox` files - a JSX-like syntax for Go.

## Features

- Syntax highlighting for `.gox` files
- Format on save
- LSP support (via gox binary)

## Requirements

Install the gox CLI:

```bash
go install github.com/germtb/gox/cmd/gox@latest
```

## Configuration

- `gox.lsp.path`: Path to the gox executable (default: `gox`)
- `gox.formatOnSave`: Format .gox files on save (default: `true`)

## Learn More

See the [gox repository](https://github.com/germtb/gox) for documentation.
