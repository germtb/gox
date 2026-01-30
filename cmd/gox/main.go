// gox is a preprocessor that transforms .gox files containing Go + JSX syntax
// into pure .go files.
package main

import (
	"fmt"
	"os"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "generate":
		if err := runGenerate(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "gox: %v\n", err)
			os.Exit(1)
		}
	case "version":
		fmt.Printf("gox version %s\n", version)
	case "help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "gox: unknown command %q\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`gox - JSX-like syntax for Go

Usage:
  gox <command> [arguments]

Commands:
  generate [path]  Generate .go files from .gox files
  version          Print version information
  help             Show this help message

Examples:
  gox generate .           Generate all .gox files in current directory
  gox generate ./ui/...    Generate .gox files recursively in ui/

Use "gox help" for more information.`)
}

func runGenerate(args []string) error {
	// TODO: Implement generate command
	if len(args) == 0 {
		args = []string{"."}
	}
	fmt.Printf("gox generate: processing %v (not yet implemented)\n", args)
	return nil
}
