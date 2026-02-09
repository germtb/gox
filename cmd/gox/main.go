// gox is a preprocessor that transforms .gox files containing Go + JSX syntax
// into pure .go files.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/germtb/gox/formatter"
	"github.com/germtb/gox/generator"
	"github.com/germtb/gox/lsp"
	"github.com/germtb/gox/parser"
)

// Build info (set by goreleaser)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	// Gox-specific commands
	switch cmd {
	case "generate":
		if err := runGenerate(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "gox: %v\n", err)
			os.Exit(1)
		}
		return
	case "fmt":
		if err := runFormat(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "gox: %v\n", err)
			os.Exit(1)
		}
		return
	case "lsp":
		if err := runLSP(); err != nil {
			fmt.Fprintf(os.Stderr, "gox: %v\n", err)
			os.Exit(1)
		}
		return
	case "version":
		fmt.Printf("gox %s\n", version)
		if commit != "none" {
			fmt.Printf("  commit: %s\n", commit)
			fmt.Printf("  built:  %s\n", date)
		}
		return
	case "help", "-h", "--help":
		printUsage()
		return
	}

	// All other commands are proxied to `go` with overlay
	if err := runGoCommand(cmd, os.Args[2:]); err != nil {
		fmt.Fprintf(os.Stderr, "gox: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`gox - JSX-like syntax for Go

Usage:
  gox <go-command> [arguments]    Proxy to 'go' with .gox overlay
  gox <gox-command> [arguments]   Run gox-specific command

Go Commands (proxied with overlay):
  gox run ./...                   Run with automatic .gox compilation
  gox build -o app ./...          Build with automatic .gox compilation
  gox test ./...                  Test with automatic .gox compilation
  gox vet ./...                   Vet with automatic .gox compilation
  (any go command works)

Gox Commands:
  generate [path]    Generate .go files from .gox files
  fmt [path]         Format .gox files
  lsp                Start LSP server (for IDE integration)
  version            Print version information
  help               Show this help message

Examples:
  gox run ./demo/                      Run a gox project
  gox test ./...                       Test all packages
  gox build -o myapp ./cmd/myapp/      Build a gox project
  gox vet ./...                        Run go vet on gox code

Format Examples:
  gox fmt .                            Format all .gox files in current directory
  gox fmt ./ui/...                     Format .gox files recursively in ui/
  gox fmt -w .                         Format and write changes to files

Generate Options:
  -o <dir>           Output directory (default: same as input)
  -runtime <pkg>     Runtime package path (default: github.com/germtb/gox)
  -parallel <n>      Number of parallel workers (default: 4)
  -overlay           Output overlay JSON instead of writing files
  -v                 Verbose output

Use "gox help" for more information.`)
}

type generateConfig struct {
	outputDir        string
	runtimePkg       string
	parallel         int
	verbose          bool
	overlay          bool   // Output overlay JSON instead of files
	overlayFile      string // Output overlay JSON to this file (default: stdout)
	tempDir          string // Temp directory for overlay files (if empty, one is created)
	paths            []string
	inMemoryMaps     bool                            // Store source maps in memory instead of writing to disk
	sourceMapsOutput map[string]*generator.SourceMap // Populated when inMemoryMaps is true
}

func runGenerate(args []string) error {
	cfg := &generateConfig{
		parallel: 4,
	}

	fs := flag.NewFlagSet("generate", flag.ExitOnError)
	fs.StringVar(&cfg.outputDir, "o", "", "output directory")
	fs.StringVar(&cfg.runtimePkg, "runtime", "", "runtime package path")
	fs.IntVar(&cfg.parallel, "parallel", 4, "number of parallel workers")
	fs.BoolVar(&cfg.verbose, "v", false, "verbose output")
	fs.BoolVar(&cfg.overlay, "overlay", false, "output go build overlay JSON (no files written to source dir)")
	fs.StringVar(&cfg.overlayFile, "overlay-file", "", "write overlay JSON to file (default: stdout)")

	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg.paths = fs.Args()
	if len(cfg.paths) == 0 {
		cfg.paths = []string{"."}
	}

	// Find all .gox files
	files, err := findGoxFiles(cfg.paths)
	if err != nil {
		return fmt.Errorf("finding files: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No .gox files found")
		return nil
	}

	if cfg.verbose {
		fmt.Printf("Found %d .gox file(s)\n", len(files))
	}

	// Process files
	if cfg.overlay {
		return processFilesOverlay(files, cfg)
	}
	return processFiles(files, cfg)
}

// skipDir returns true for directories that should be skipped during recursive walks.
// This mirrors Go's own ./... behavior of skipping dot-prefixed and underscore-prefixed
// directories, plus common non-Go directories.
func skipDir(name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
		return true
	}
	switch name {
	case "vendor", "testdata", "node_modules":
		return true
	}
	return false
}

// findGoxFiles finds all .gox files in the given paths.
func findGoxFiles(paths []string) ([]string, error) {
	var files []string

	for _, path := range paths {
		// Handle recursive pattern ./...
		if strings.HasSuffix(path, "/...") {
			dir := strings.TrimSuffix(path, "/...")
			if dir == "" {
				dir = "."
			}
			err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if info.IsDir() && p != dir && skipDir(info.Name()) {
					return filepath.SkipDir
				}
				if !info.IsDir() && strings.HasSuffix(p, ".gox") {
					files = append(files, p)
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
			continue
		}

		// Check if path is a file or directory
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", path, err)
		}

		if info.IsDir() {
			// Find all .gox files in directory (non-recursive)
			entries, err := os.ReadDir(path)
			if err != nil {
				return nil, fmt.Errorf("reading directory %s: %w", path, err)
			}
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".gox") {
					files = append(files, filepath.Join(path, entry.Name()))
				}
			}
		} else if strings.HasSuffix(path, ".gox") {
			files = append(files, path)
		}
	}

	return files, nil
}

// processFiles generates Go code for all input files.
func processFiles(files []string, cfg *generateConfig) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(files))
	semaphore := make(chan struct{}, cfg.parallel)

	for _, file := range files {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			semaphore <- struct{}{}        // Acquire
			defer func() { <-semaphore }() // Release

			if err := processFile(f, cfg); err != nil {
				errChan <- fmt.Errorf("%s: %w", f, err)
			}
		}(file)
	}

	wg.Wait()
	close(errChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		return fmt.Errorf("%d file(s) failed", len(errs))
	}

	return nil
}

// processFile generates Go code for a single .gox file.
func processFile(inputPath string, cfg *generateConfig) error {
	if cfg.verbose {
		fmt.Printf("Processing %s\n", inputPath)
	}

	// Read input file
	src, err := os.ReadFile(inputPath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	// Parse
	file, err := parser.Parse(inputPath, src)
	if err != nil {
		return fmt.Errorf("parsing: %w", err)
	}

	// Generate
	opts := &generator.Options{}
	if cfg.runtimePkg != "" {
		opts.RuntimePackage = cfg.runtimePkg
	}

	output, sourceMap, err := generator.Generate(file, opts)
	if err != nil {
		return fmt.Errorf("generating: %w", err)
	}

	// Determine output path
	outputPath := getOutputPath(inputPath, cfg.outputDir)

	// Set source map file paths
	absInputPath, _ := filepath.Abs(inputPath)
	absOutputPath, _ := filepath.Abs(outputPath)
	sourceMap.SetFiles(absInputPath, absOutputPath)

	// Write output file
	if err := os.WriteFile(outputPath, output, 0644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}

	// Write source map file
	sourceMapPath := outputPath + ".map"
	sourceMapData, err := sourceMap.ToJSON()
	if err != nil {
		return fmt.Errorf("serializing source map: %w", err)
	}
	if err := os.WriteFile(sourceMapPath, sourceMapData, 0644); err != nil {
		return fmt.Errorf("writing source map: %w", err)
	}

	if cfg.verbose {
		fmt.Printf("  -> %s\n", outputPath)
		fmt.Printf("  -> %s\n", sourceMapPath)
	}

	return nil
}

// getOutputPath determines the output path for a .gox file.
// Test files get special handling: foo_test.gox → foo_gox_test.go
// so that Go's test runner recognizes them.
func getOutputPath(inputPath, outputDir string) string {
	baseName := strings.TrimSuffix(filepath.Base(inputPath), ".gox")

	var outputName string
	if strings.HasSuffix(baseName, "_test") {
		// foo_test.gox → foo_gox_test.go
		outputName = strings.TrimSuffix(baseName, "_test") + "_gox_test.go"
	} else {
		// foo.gox → foo_gox.go
		outputName = baseName + "_gox.go"
	}

	if outputDir != "" {
		return filepath.Join(outputDir, outputName)
	}

	// Same directory as input
	return filepath.Join(filepath.Dir(inputPath), outputName)
}

// Overlay represents the Go build overlay JSON format.
type Overlay struct {
	Replace map[string]string `json:"Replace"`
}

// processFilesOverlay generates code and outputs overlay JSON.
func processFilesOverlay(files []string, cfg *generateConfig) error {
	// Use provided temp directory or create one
	tempDir := cfg.tempDir
	if tempDir == "" {
		var err error
		tempDir, err = os.MkdirTemp("", "gox-overlay-*")
		if err != nil {
			return fmt.Errorf("creating temp dir: %w", err)
		}
	}

	if cfg.verbose {
		fmt.Fprintf(os.Stderr, "Using temp dir: %s\n", tempDir)
	}

	overlay := Overlay{
		Replace: make(map[string]string),
	}

	// Initialize source maps output if using in-memory maps
	if cfg.inMemoryMaps && cfg.sourceMapsOutput == nil {
		cfg.sourceMapsOutput = make(map[string]*generator.SourceMap)
	}

	cwd, _ := os.Getwd()

	// Process each file
	for _, inputPath := range files {
		if cfg.verbose {
			fmt.Fprintf(os.Stderr, "Processing %s\n", inputPath)
		}

		// Read and parse
		src, err := os.ReadFile(inputPath)
		if err != nil {
			return fmt.Errorf("%s: reading file: %w", inputPath, err)
		}

		file, err := parser.Parse(inputPath, src)
		if err != nil {
			return fmt.Errorf("%s: parsing: %w", inputPath, err)
		}

		// Generate
		opts := &generator.Options{}
		if cfg.runtimePkg != "" {
			opts.RuntimePackage = cfg.runtimePkg
		}

		output, sourceMap, err := generator.Generate(file, opts)
		if err != nil {
			return fmt.Errorf("%s: generating: %w", inputPath, err)
		}

		// Write to temp file
		absInput, err := filepath.Abs(inputPath)
		if err != nil {
			return fmt.Errorf("%s: getting absolute path: %w", inputPath, err)
		}

		// Target path (where the file would normally go)
		targetPath := getOutputPath(absInput, "")

		// Set source map file paths
		sourceMap.SetFiles(absInput, targetPath)

		// Temp file path - preserve directory structure to avoid collisions
		// when multiple packages have .gox files with the same base name.
		relTarget, relErr := filepath.Rel(cwd, targetPath)
		if relErr != nil {
			relTarget = filepath.Base(targetPath)
		}
		tempFile := filepath.Join(tempDir, relTarget)
		if err := os.MkdirAll(filepath.Dir(tempFile), 0755); err != nil {
			return fmt.Errorf("%s: creating temp subdir: %w", inputPath, err)
		}
		if err := os.WriteFile(tempFile, output, 0644); err != nil {
			return fmt.Errorf("%s: writing temp file: %w", inputPath, err)
		}

		// Handle source maps: either in memory or on disk
		if cfg.inMemoryMaps {
			// Store in memory for error remapping
			cfg.sourceMapsOutput[targetPath] = sourceMap
			cfg.sourceMapsOutput[tempFile] = sourceMap
		} else {
			// Write source map to temp dir
			sourceMapPath := tempFile + ".map"
			sourceMapData, err := sourceMap.ToJSON()
			if err != nil {
				return fmt.Errorf("%s: serializing source map: %w", inputPath, err)
			}
			if err := os.WriteFile(sourceMapPath, sourceMapData, 0644); err != nil {
				return fmt.Errorf("%s: writing source map: %w", inputPath, err)
			}
		}

		// Add to overlay mapping
		overlay.Replace[targetPath] = tempFile

		if cfg.verbose {
			fmt.Fprintf(os.Stderr, "  %s -> %s\n", targetPath, tempFile)
		}
	}

	// Output overlay JSON
	jsonBytes, err := json.MarshalIndent(overlay, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling overlay: %w", err)
	}

	if cfg.overlayFile != "" {
		if err := os.WriteFile(cfg.overlayFile, jsonBytes, 0644); err != nil {
			return fmt.Errorf("writing overlay file: %w", err)
		}
		if cfg.verbose {
			fmt.Fprintf(os.Stderr, "Overlay written to %s\n", cfg.overlayFile)
		}
	} else {
		fmt.Println(string(jsonBytes))
	}

	return nil
}

// isPath checks if an argument looks like a file or directory path
func isPath(arg string) bool {
	if strings.HasPrefix(arg, "./") || strings.HasPrefix(arg, "/") || arg == "." {
		return true
	}
	// .gox files are always treated as paths
	if strings.HasSuffix(arg, ".gox") {
		return true
	}
	return false
}

// runGoCommand runs go run or go build with automatic overlay generation.
func runGoCommand(goCmd string, args []string) error {
	// Extract paths and flags from args.
	// For "go run", arguments after the package path are program arguments.
	var paths []string
	var goArgs []string
	var programArgs []string
	foundPath := false

	for i := 0; i < len(args); i++ {
		arg := args[i]

		// Once we've found a path for "go run", everything else is a program arg
		if foundPath && goCmd == "run" && !strings.HasPrefix(arg, "-") {
			programArgs = append(programArgs, args[i:]...)
			break
		}

		if strings.HasPrefix(arg, "-") {
			goArgs = append(goArgs, arg)
			// Check if this flag takes a value (common go build/run flags)
			if arg == "-o" || arg == "-p" || arg == "-exec" ||
				arg == "-ldflags" || arg == "-tags" || arg == "-mod" ||
				arg == "-cover" || arg == "-coverprofile" {
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					i++
					goArgs = append(goArgs, args[i])
				}
			}
		} else if isPath(arg) {
			paths = append(paths, arg)
			foundPath = true
		} else {
			// Could be a package path like "mypackage" or program args for go run
			goArgs = append(goArgs, arg)
		}
	}

	if len(paths) == 0 {
		paths = []string{"."}
	}

	// Convert .gox file paths to directory paths for go run/build
	var goPaths []string
	for _, p := range paths {
		if strings.HasSuffix(p, ".gox") {
			dir := filepath.Dir(p)
			// Ensure ./ prefix for relative paths (required by go run)
			if !strings.HasPrefix(dir, "./") && !strings.HasPrefix(dir, "/") && dir != "." {
				dir = "./" + dir
			}
			goPaths = append(goPaths, dir)
		} else {
			goPaths = append(goPaths, p)
		}
	}

	// Find all .gox files recursively from the project root.
	// We always scan ./... so that .gox files in dependency packages
	// (e.g., cmd/status.gox imported by main.go) are included in the overlay,
	// even when the build target is just ".".
	goxFiles, err := findGoxFiles([]string{"./..."})
	if err != nil {
		return fmt.Errorf("finding gox files: %w", err)
	}

	// If no .gox files, just run go command directly
	if len(goxFiles) == 0 {
		cmd := exec.Command("go", append([]string{goCmd}, args...)...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		return cmd.Run()
	}

	// Generate overlay to temp file
	overlayFile, err := os.CreateTemp("", "gox-overlay-*.json")
	if err != nil {
		return fmt.Errorf("creating overlay file: %w", err)
	}
	defer os.Remove(overlayFile.Name())
	overlayFile.Close()

	// Create temp directory for generated files
	tempDir, err := os.MkdirTemp("", "gox-overlay-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	cfg := &generateConfig{
		overlay:      true,
		overlayFile:  overlayFile.Name(),
		tempDir:      tempDir,
		paths:        paths,
		inMemoryMaps: true,
	}

	if err := processFilesOverlay(goxFiles, cfg); err != nil {
		return fmt.Errorf("generating overlay: %w", err)
	}

	// Write generated files to source directories for vet compatibility.
	// go vet doesn't read files through the overlay, so the files must exist
	// on disk. These are gitignored via *_gox.go / *_gox_test.go and cleaned up after the command.
	overlayData, err := os.ReadFile(overlayFile.Name())
	if err != nil {
		return fmt.Errorf("reading overlay: %w", err)
	}
	var overlayJSON Overlay
	if err := json.Unmarshal(overlayData, &overlayJSON); err != nil {
		return fmt.Errorf("parsing overlay: %w", err)
	}
	var createdFiles []string
	for targetPath, tempPath := range overlayJSON.Replace {
		// Only track files we create, not ones that already exist on disk
		// (e.g., from a previous "gox generate").
		if _, statErr := os.Stat(targetPath); statErr == nil {
			continue
		}
		content, readErr := os.ReadFile(tempPath)
		if readErr != nil {
			continue
		}
		if writeErr := os.WriteFile(targetPath, content, 0644); writeErr != nil {
			continue
		}
		createdFiles = append(createdFiles, targetPath)
	}
	defer func() {
		for _, f := range createdFiles {
			os.Remove(f)
		}
	}()

	// Build go command with overlay
	cmdArgs := []string{goCmd, "-overlay=" + overlayFile.Name()}
	cmdArgs = append(cmdArgs, goArgs...)
	cmdArgs = append(cmdArgs, goPaths...)
	cmdArgs = append(cmdArgs, programArgs...)

	cmd := exec.Command("go", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin

	// Capture stderr for error remapping
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	err = cmd.Run()

	// Remap and output errors using in-memory source maps
	if stderrBuf.Len() > 0 {
		remapped := remapErrors(stderrBuf.String(), cfg.sourceMapsOutput)
		fmt.Fprint(os.Stderr, remapped)
	}

	return err
}

// errorPattern matches Go compiler error format: file.go:line:col: message
var errorPattern = regexp.MustCompile(`^(.+\.go):(\d+):(\d+):(.*)$`)

// remapErrors takes go build/run stderr output and remaps _gox.go errors to .gox locations
func remapErrors(stderr string, sourceMaps map[string]*generator.SourceMap) string {
	var result strings.Builder
	scanner := bufio.NewScanner(strings.NewReader(stderr))

	for scanner.Scan() {
		line := scanner.Text()
		remapped := remapErrorLine(line, sourceMaps)
		result.WriteString(remapped)
		result.WriteString("\n")
	}

	return result.String()
}

// remapErrorLine remaps a single error line
func remapErrorLine(line string, sourceMaps map[string]*generator.SourceMap) string {
	matches := errorPattern.FindStringSubmatch(line)
	if matches == nil {
		return line
	}

	filePath := matches[1]
	lineNum, _ := strconv.Atoi(matches[2])
	colNum, _ := strconv.Atoi(matches[3])
	message := matches[4]

	// Check if this is a generated gox file (_gox.go or _gox_test.go)
	if !strings.HasSuffix(filePath, "_gox.go") && !strings.HasSuffix(filePath, "_gox_test.go") {
		return line
	}

	// Look up source map in memory
	sm, ok := sourceMaps[filePath]
	if !ok {
		return line // No mapping found, return original
	}

	// Remap position (Go compiler uses 1-indexed, source map uses 0-indexed)
	srcPos, ok := sm.SourcePositionFromTarget(uint32(lineNum-1), uint32(colNum-1))
	if !ok {
		return line // No mapping found
	}

	// Output remapped error with .gox file
	return fmt.Sprintf("%s:%d:%d:%s", sm.SourceFile, srcPos.Line+1, srcPos.Column+1, message)
}

// runLSP starts the LSP server.
func runLSP() error {
	proxy, err := lsp.New()
	if err != nil {
		return err
	}
	defer proxy.Close()
	return proxy.Run()
}

// formatConfig holds configuration for the format command.
type formatConfig struct {
	write   bool // Write result to file instead of stdout
	diff    bool // Show diff instead of formatted output
	list    bool // List files that would be formatted
	verbose bool
	paths   []string
}

// runFormat runs the format command.
func runFormat(args []string) error {
	cfg := &formatConfig{}

	fs := flag.NewFlagSet("fmt", flag.ExitOnError)
	fs.BoolVar(&cfg.write, "w", false, "write result to file instead of stdout")
	fs.BoolVar(&cfg.diff, "d", false, "display diff instead of formatted output")
	fs.BoolVar(&cfg.list, "l", false, "list files that would be formatted")
	fs.BoolVar(&cfg.verbose, "v", false, "verbose output")

	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg.paths = fs.Args()
	if len(cfg.paths) == 0 {
		cfg.paths = []string{"."}
	}

	// Find all .gox files
	files, err := findGoxFiles(cfg.paths)
	if err != nil {
		return fmt.Errorf("finding files: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No .gox files found")
		return nil
	}

	// Process each file
	var hasChanges bool
	for _, file := range files {
		changed, err := formatFile(file, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", file, err)
			continue
		}
		if changed {
			hasChanges = true
		}
	}

	if cfg.list && !hasChanges {
		if cfg.verbose {
			fmt.Println("All files are properly formatted")
		}
	}

	return nil
}

// formatFile formats a single .gox file.
func formatFile(path string, cfg *formatConfig) (bool, error) {
	// Read source
	src, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("reading file: %w", err)
	}

	// Parse
	file, err := parser.Parse(path, src)
	if err != nil {
		return false, fmt.Errorf("parsing: %w", err)
	}

	// Format
	formatted, err := formatter.Format(file, nil)
	if err != nil {
		return false, fmt.Errorf("formatting: %w", err)
	}

	// Check if changed
	changed := !bytes.Equal(src, formatted)

	if cfg.list {
		if changed {
			fmt.Println(path)
		}
		return changed, nil
	}

	if cfg.diff {
		if changed {
			// Simple diff output
			fmt.Printf("--- %s (original)\n", path)
			fmt.Printf("+++ %s (formatted)\n", path)
			fmt.Println(string(formatted))
		}
		return changed, nil
	}

	if cfg.write {
		if changed {
			if err := os.WriteFile(path, formatted, 0644); err != nil {
				return false, fmt.Errorf("writing file: %w", err)
			}
			if cfg.verbose {
				fmt.Printf("Formatted %s\n", path)
			}
		} else if cfg.verbose {
			fmt.Printf("Unchanged %s\n", path)
		}
		return changed, nil
	}

	// Default: output to stdout
	fmt.Print(string(formatted))
	return changed, nil
}
