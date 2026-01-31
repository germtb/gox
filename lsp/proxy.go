// Package lsp implements a Language Server Protocol proxy for gox files.
// It proxies requests to gopls while translating positions between .gox and generated .go files.
package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/germtb/gox/formatter"
	"github.com/germtb/gox/generator"
	"github.com/germtb/gox/parser"
)

// Proxy is an LSP proxy that sits between the editor and gopls.
type Proxy struct {
	gopls        *exec.Cmd
	goplsIn      io.WriteCloser
	goplsOut     io.ReadCloser
	sourceMaps   map[string]*generator.SourceMap // .gox path -> source map
	fileContents map[string]string               // .gox path -> current content
	tempDir      string
	mu           sync.RWMutex
	log          *log.Logger
}

// New creates a new LSP proxy.
func New() (*Proxy, error) {
	tempDir, err := os.MkdirTemp("", "gox-lsp-*")
	if err != nil {
		return nil, fmt.Errorf("creating temp dir: %w", err)
	}

	// Create log file
	logPath := filepath.Join(tempDir, "gox-lsp.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		// Fall back to stderr
		log.Printf("gox-lsp: couldn't create log file %s: %v, using stderr", logPath, err)
		return &Proxy{
			sourceMaps:   make(map[string]*generator.SourceMap),
			fileContents: make(map[string]string),
			tempDir:      tempDir,
			log:          log.New(os.Stderr, "[gox-lsp] ", log.LstdFlags|log.Lshortfile),
		}, nil
	}

	logger := log.New(logFile, "[gox-lsp] ", log.LstdFlags|log.Lshortfile)
	logger.Printf("Starting gox LSP proxy, temp dir: %s", tempDir)

	return &Proxy{
		sourceMaps:   make(map[string]*generator.SourceMap),
		fileContents: make(map[string]string),
		tempDir:      tempDir,
		log:          logger,
	}, nil
}

// Run starts the proxy, reading from stdin and writing to stdout.
func (p *Proxy) Run() error {
	// Find gopls - check common locations
	goplsPath := findGopls()
	if goplsPath == "" {
		return fmt.Errorf("gopls not found. Install with: go install golang.org/x/tools/gopls@latest")
	}

	p.log.Printf("Found gopls at: %s", goplsPath)

	// Start gopls
	p.gopls = exec.Command(goplsPath, "serve")
	var err error
	p.goplsIn, err = p.gopls.StdinPipe()
	if err != nil {
		return fmt.Errorf("gopls stdin: %w", err)
	}
	p.goplsOut, err = p.gopls.StdoutPipe()
	if err != nil {
		return fmt.Errorf("gopls stdout: %w", err)
	}
	// Capture gopls stderr to our log
	p.gopls.Stderr = p.log.Writer()

	if err := p.gopls.Start(); err != nil {
		return fmt.Errorf("starting gopls: %w", err)
	}
	p.log.Printf("Started gopls (pid %d)", p.gopls.Process.Pid)

	// Proxy in both directions concurrently
	done := make(chan error, 2)

	go func() {
		p.proxyToGopls(os.Stdin)
		done <- nil
	}()

	go func() {
		p.proxyFromGopls(os.Stdout)
		done <- nil
	}()

	// Wait for either direction to finish
	<-done

	p.gopls.Process.Kill()
	return nil
}

// findGopls looks for gopls in PATH and common locations.
func findGopls() string {
	// Try PATH first
	if path, err := exec.LookPath("gopls"); err == nil {
		return path
	}

	// Try common Go bin locations
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, "go", "bin", "gopls"),
		"/usr/local/go/bin/gopls",
		"/usr/local/bin/gopls",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// proxyToGopls reads LSP messages from the editor and forwards to gopls.
func (p *Proxy) proxyToGopls(r io.Reader) {
	p.log.Printf("Started reading from editor")
	reader := bufio.NewReader(r)
	for {
		msg, err := readMessage(reader)
		if err != nil {
			p.log.Printf("Read error from editor: %v", err)
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "gox-lsp: read error: %v\n", err)
			}
			return
		}

		p.log.Printf("Received message (%d bytes)", len(msg))

		// Check if we should handle this request ourselves
		if response := p.handleRequestDirectly(msg); response != nil {
			// Write response directly to editor (stdout)
			if err := writeMessage(os.Stdout, response); err != nil {
				p.log.Printf("Write error to editor: %v", err)
			}
			continue
		}

		// Rewrite .gox URIs and positions to .go
		rewritten := p.rewriteToGo(msg)

		// Forward to gopls
		if err := writeMessage(p.goplsIn, rewritten); err != nil {
			p.log.Printf("Write error to gopls: %v", err)
			fmt.Fprintf(os.Stderr, "gox-lsp: write error: %v\n", err)
			return
		}
	}
}

// proxyFromGopls reads LSP messages from gopls and forwards to the editor.
func (p *Proxy) proxyFromGopls(w io.Writer) {
	p.log.Printf("Started reading from gopls")
	reader := bufio.NewReader(p.goplsOut)
	for {
		msg, err := readMessage(reader)
		if err != nil {
			p.log.Printf("Read error from gopls: %v", err)
			if err != io.EOF {
				fmt.Fprintf(os.Stderr, "gox-lsp: gopls read error: %v\n", err)
			}
			return
		}

		p.log.Printf("Received from gopls (%d bytes)", len(msg))

		// Rewrite .go URIs and positions back to .gox
		rewritten := p.rewriteToGox(msg)

		// Forward to editor
		if err := writeMessage(w, rewritten); err != nil {
			p.log.Printf("Write error to editor: %v", err)
			fmt.Fprintf(os.Stderr, "gox-lsp: editor write error: %v\n", err)
			return
		}
	}
}

// rewriteToGo rewrites a message from editor, translating .gox to .go.
func (p *Proxy) rewriteToGo(msg []byte) []byte {
	var obj map[string]any
	if err := json.Unmarshal(msg, &obj); err != nil {
		return msg
	}

	// Log the method
	if method, ok := obj["method"].(string); ok {
		p.log.Printf("-> %s", method)

		switch method {
		case "textDocument/didOpen":
			p.handleDidOpen(obj)
		case "textDocument/didChange":
			p.handleDidChange(obj)
		case "textDocument/didClose":
			p.handleDidClose(obj)
		}
	}

	// Translate positions from .gox to .go BEFORE rewriting URIs
	p.translatePositionsToGo(obj)

	// Rewrite URIs in the message
	p.rewriteURIs(obj, true)

	result, _ := json.Marshal(obj)
	return result
}

// rewriteToGox rewrites a message from gopls, translating .go back to .gox.
func (p *Proxy) rewriteToGox(msg []byte) []byte {
	var obj map[string]any
	if err := json.Unmarshal(msg, &obj); err != nil {
		return msg
	}

	// Log response details for debugging
	if id, ok := obj["id"]; ok {
		if result, ok := obj["result"]; ok {
			p.log.Printf("<- response id=%v result_type=%T", id, result)
			if resultBytes, err := json.Marshal(result); err == nil && len(resultBytes) < 500 {
				p.log.Printf("   result: %s", string(resultBytes))
			}
		}
		if errObj, ok := obj["error"]; ok {
			p.log.Printf("<- error id=%v: %v", id, errObj)
		}
	}
	if method, ok := obj["method"].(string); ok {
		p.log.Printf("<- notification: %s", method)
	}

	// Rewrite URIs and positions
	p.rewriteURIs(obj, false)
	p.rewritePositions(obj)

	result, _ := json.Marshal(obj)
	return result
}

// handleDidOpen generates .go file, caches source map, and replaces content in message.
func (p *Proxy) handleDidOpen(msg map[string]any) {
	params, ok := msg["params"].(map[string]any)
	if !ok {
		return
	}
	textDoc, ok := params["textDocument"].(map[string]any)
	if !ok {
		return
	}
	uri, ok := textDoc["uri"].(string)
	if !ok || !strings.HasSuffix(uri, ".gox") {
		return
	}
	text, ok := textDoc["text"].(string)
	if !ok {
		return
	}

	// Store the original .gox content for formatting
	goxPath := uriToPath(uri)
	p.mu.Lock()
	p.fileContents[goxPath] = text
	p.mu.Unlock()

	// Generate .go file and get the content
	goContent := p.generateAndCache(uri, text)
	if goContent != "" {
		// Replace the text content with generated Go code
		textDoc["text"] = goContent
		textDoc["languageId"] = "go"
		p.log.Printf("Replaced didOpen content with generated Go (%d bytes)", len(goContent))
	}
}

// handleDidChange regenerates .go file on changes.
func (p *Proxy) handleDidChange(msg map[string]any) {
	params, ok := msg["params"].(map[string]any)
	if !ok {
		return
	}
	textDoc, ok := params["textDocument"].(map[string]any)
	if !ok {
		return
	}
	uri, ok := textDoc["uri"].(string)
	if !ok || !strings.HasSuffix(uri, ".gox") {
		return
	}

	// Get full text from content changes
	changes, ok := params["contentChanges"].([]any)
	if !ok || len(changes) == 0 {
		return
	}
	change, ok := changes[0].(map[string]any)
	if !ok {
		return
	}
	text, ok := change["text"].(string)
	if !ok {
		return
	}

	// Store the original .gox content for formatting
	goxPath := uriToPath(uri)
	p.mu.Lock()
	p.fileContents[goxPath] = text
	p.mu.Unlock()

	goContent := p.generateAndCache(uri, text)
	if goContent != "" {
		// Replace the text content with generated Go code
		change["text"] = goContent
		p.log.Printf("Replaced didChange content with generated Go (%d bytes)", len(goContent))
	}
}

// handleDidClose cleans up cached data.
func (p *Proxy) handleDidClose(msg map[string]any) {
	params, ok := msg["params"].(map[string]any)
	if !ok {
		return
	}
	textDoc, ok := params["textDocument"].(map[string]any)
	if !ok {
		return
	}
	uri, ok := textDoc["uri"].(string)
	if !ok || !strings.HasSuffix(uri, ".gox") {
		return
	}

	p.mu.Lock()
	delete(p.sourceMaps, uriToPath(uri))
	p.mu.Unlock()
}

// generateAndCache parses .gox, generates .go, and caches the source map.
// Returns the generated Go content, or empty string on error.
func (p *Proxy) generateAndCache(uri, text string) string {
	goxPath := uriToPath(uri)

	// Lock during the entire generation to prevent races
	p.mu.Lock()
	defer p.mu.Unlock()

	p.log.Printf("Generating .go for: %s (%d bytes)", goxPath, len(text))

	// Parse
	file, err := parser.Parse(goxPath, []byte(text))
	if err != nil {
		p.log.Printf("Parse error: %v", err)
		return ""
	}

	// Generate
	output, sourceMap, err := generator.Generate(file, nil)
	if err != nil {
		p.log.Printf("Generate error: %v", err)
		return ""
	}

	// Write to file (for gopls workspace)
	goPath := p.goxToGoPath(goxPath)
	sourceMap.SetFiles(goxPath, goPath)

	if err := os.MkdirAll(filepath.Dir(goPath), 0755); err != nil {
		p.log.Printf("Mkdir error: %v", err)
		return ""
	}
	if err := os.WriteFile(goPath, output, 0644); err != nil {
		p.log.Printf("Write error: %v", err)
		return ""
	}

	p.log.Printf("Generated: %s -> %s (%d bytes)", goxPath, goPath, len(output))

	// Cache source map
	p.sourceMaps[goxPath] = sourceMap

	return string(output)
}

// goxToGoPath converts a .gox path to the generated .go path.
// The .go file is placed next to the .gox file for same-package context.
func (p *Proxy) goxToGoPath(goxPath string) string {
	// e.g., /path/to/app.gox -> /path/to/app_gox.go
	return strings.TrimSuffix(goxPath, ".gox") + "_gox.go"
}

// rewriteURIs rewrites file URIs in a message.
// toGo=true: .gox -> .go, toGo=false: .go -> .gox
func (p *Proxy) rewriteURIs(obj any, toGo bool) {
	switch v := obj.(type) {
	case map[string]any:
		for key, val := range v {
			if key == "uri" || key == "targetUri" {
				if uri, ok := val.(string); ok {
					if toGo && strings.HasSuffix(uri, ".gox") {
						goxPath := uriToPath(uri)
						goPath := p.goxToGoPath(goxPath)
						v[key] = pathToURI(goPath)
					} else if !toGo && strings.HasSuffix(uri, "_gox.go") {
						// Find original .gox file from source map
						goPath := uriToPath(uri)
						p.mu.RLock()
						for goxPath, sm := range p.sourceMaps {
							if sm.TargetFile == goPath {
								v[key] = pathToURI(goxPath)
								break
							}
						}
						p.mu.RUnlock()
					}
				}
			} else {
				p.rewriteURIs(val, toGo)
			}
		}
	case []any:
		for _, item := range v {
			p.rewriteURIs(item, toGo)
		}
	}
}

// translatePositionsToGo translates positions from .gox to .go coordinates.
func (p *Proxy) translatePositionsToGo(obj any) {
	switch v := obj.(type) {
	case map[string]any:
		// Look for textDocument with a .gox URI
		if textDoc, ok := v["textDocument"].(map[string]any); ok {
			if uri, ok := textDoc["uri"].(string); ok && strings.HasSuffix(uri, ".gox") {
				goxPath := uriToPath(uri)
				p.mu.RLock()
				sm := p.sourceMaps[goxPath]
				p.mu.RUnlock()

				if sm != nil {
					// Translate position field
					if pos, ok := v["position"].(map[string]any); ok {
						p.translatePositionToGoLine(pos, sm)
					}
					// Translate range field
					if rng, ok := v["range"].(map[string]any); ok {
						if start, ok := rng["start"].(map[string]any); ok {
							p.translatePositionToGoLine(start, sm)
						}
						if end, ok := rng["end"].(map[string]any); ok {
							p.translatePositionToGoLine(end, sm)
						}
					}
				}
			}
		}

		// Recurse
		for _, val := range v {
			p.translatePositionsToGo(val)
		}
	case []any:
		for _, item := range v {
			p.translatePositionsToGo(item)
		}
	}
}

// translatePositionToGoLine translates a position using line-level mapping.
// We find any mapping on the source line and use its target line, preserving relative column.
func (p *Proxy) translatePositionToGoLine(pos map[string]any, sm *generator.SourceMap) {
	line, ok1 := pos["line"].(float64)
	char, ok2 := pos["character"].(float64)
	if !ok1 || !ok2 {
		return
	}

	srcLine := uint32(line)

	// Find any mapping on this line to get the target line
	targetLine, found := sm.FindTargetLine(srcLine)
	if found {
		p.log.Printf("Line translate: %d -> %d (col %d)", srcLine, targetLine, int(char))
		pos["line"] = float64(targetLine)
	}
}

// rewritePositions rewrites line/column positions from .go back to .gox using line-level mapping.
func (p *Proxy) rewritePositions(obj any) {
	switch v := obj.(type) {
	case map[string]any:
		// Check if this object has a uri that maps to a .gox file
		if uri, ok := v["uri"].(string); ok && strings.HasSuffix(uri, ".gox") {
			goxPath := uriToPath(uri)
			p.mu.RLock()
			sm := p.sourceMaps[goxPath]
			p.mu.RUnlock()

			if sm != nil {
				// Rewrite position using line mapping
				if pos, ok := v["position"].(map[string]any); ok {
					p.rewritePositionLine(pos, sm)
				}
				// Rewrite range using line mapping
				if rng, ok := v["range"].(map[string]any); ok {
					if start, ok := rng["start"].(map[string]any); ok {
						p.rewritePositionLine(start, sm)
					}
					if end, ok := rng["end"].(map[string]any); ok {
						p.rewritePositionLine(end, sm)
					}
				}
			}
		}

		// Recurse
		for _, val := range v {
			p.rewritePositions(val)
		}
	case []any:
		for _, item := range v {
			p.rewritePositions(item)
		}
	}
}

// rewritePositionLine rewrites a position using line-level mapping from .go back to .gox.
func (p *Proxy) rewritePositionLine(pos map[string]any, sm *generator.SourceMap) {
	line, ok1 := pos["line"].(float64)
	char, ok2 := pos["character"].(float64)
	if !ok1 || !ok2 {
		return
	}

	tgtLine := uint32(line)
	if srcLine, found := sm.FindSourceLine(tgtLine); found {
		pos["line"] = float64(srcLine)
		// Keep column as-is for now
		p.log.Printf("Response line translate: %d -> %d (col %d)", tgtLine, srcLine, int(char))
	}
}

// LSP message helpers

func readMessage(r *bufio.Reader) ([]byte, error) {
	// Read headers
	var contentLength int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break // End of headers
		}
		if strings.HasPrefix(line, "Content-Length:") {
			length := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			contentLength, _ = strconv.Atoi(length)
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("no Content-Length header")
	}

	// Read body
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}

	return body, nil
}

func writeMessage(w io.Writer, body []byte) error {
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := w.Write([]byte(header)); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

func uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return strings.TrimPrefix(uri, "file://")
	}
	return uri
}

func pathToURI(path string) string {
	if !strings.HasPrefix(path, "file://") {
		return "file://" + path
	}
	return path
}

// Close cleans up resources.
func (p *Proxy) Close() error {
	if p.gopls != nil && p.gopls.Process != nil {
		p.gopls.Process.Kill()
	}
	return os.RemoveAll(p.tempDir)
}

// handleRequestDirectly checks if we should handle a request ourselves instead of forwarding to gopls.
// Returns a response to send to the editor, or nil if the request should be forwarded.
func (p *Proxy) handleRequestDirectly(msg []byte) []byte {
	var obj map[string]any
	if err := json.Unmarshal(msg, &obj); err != nil {
		return nil
	}

	method, ok := obj["method"].(string)
	if !ok {
		return nil
	}

	// Handle formatting requests for .gox files
	if method == "textDocument/formatting" {
		return p.handleFormatting(obj)
	}

	// Handle codeAction requests for .gox files
	if method == "textDocument/codeAction" {
		return p.handleCodeAction(obj)
	}

	return nil
}

// handleFormatting handles textDocument/formatting requests for .gox files.
func (p *Proxy) handleFormatting(req map[string]any) []byte {
	id := req["id"]
	params, ok := req["params"].(map[string]any)
	if !ok {
		return p.makeErrorResponse(id, -32602, "Invalid params")
	}

	textDoc, ok := params["textDocument"].(map[string]any)
	if !ok {
		return p.makeErrorResponse(id, -32602, "Invalid textDocument")
	}

	uri, ok := textDoc["uri"].(string)
	if !ok {
		return p.makeErrorResponse(id, -32602, "Invalid uri")
	}

	// Only handle .gox files
	if !strings.HasSuffix(uri, ".gox") {
		return nil // Let gopls handle non-.gox files
	}

	p.log.Printf("Handling formatting for %s", uri)

	goxPath := uriToPath(uri)

	// Get the current file content
	p.mu.RLock()
	content, ok := p.fileContents[goxPath]
	p.mu.RUnlock()

	if !ok {
		// Try reading from disk as fallback
		data, err := os.ReadFile(goxPath)
		if err != nil {
			return p.makeErrorResponse(id, -32603, "File not found: "+goxPath)
		}
		content = string(data)
	}

	// Parse and format
	file, err := parser.Parse(goxPath, []byte(content))
	if err != nil {
		p.log.Printf("Parse error during formatting: %v", err)
		return p.makeErrorResponse(id, -32603, "Parse error: "+err.Error())
	}

	formatted, err := formatter.Format(file, nil)
	if err != nil {
		p.log.Printf("Format error: %v", err)
		return p.makeErrorResponse(id, -32603, "Format error: "+err.Error())
	}

	// If no changes, return empty edits
	if string(formatted) == content {
		p.log.Printf("No formatting changes needed")
		return p.makeSuccessResponse(id, []any{})
	}

	// Count lines in original content
	lines := strings.Split(content, "\n")
	endLine := len(lines) - 1
	endChar := len(lines[endLine])

	// Return a single edit that replaces the entire file
	edit := map[string]any{
		"range": map[string]any{
			"start": map[string]any{"line": 0, "character": 0},
			"end":   map[string]any{"line": endLine, "character": endChar},
		},
		"newText": string(formatted),
	}

	p.log.Printf("Formatting applied (%d -> %d bytes)", len(content), len(formatted))
	return p.makeSuccessResponse(id, []any{edit})
}

// makeSuccessResponse creates a JSON-RPC success response.
func (p *Proxy) makeSuccessResponse(id any, result any) []byte {
	response := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	}
	data, _ := json.Marshal(response)
	return data
}

// makeErrorResponse creates a JSON-RPC error response.
func (p *Proxy) makeErrorResponse(id any, code int, message string) []byte {
	response := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}
	data, _ := json.Marshal(response)
	return data
}

// handleCodeAction handles textDocument/codeAction requests for .gox files.
// For now, we return an empty array (no code actions available).
func (p *Proxy) handleCodeAction(req map[string]any) []byte {
	id := req["id"]
	params, ok := req["params"].(map[string]any)
	if !ok {
		return nil // Let gopls handle it
	}

	textDoc, ok := params["textDocument"].(map[string]any)
	if !ok {
		return nil
	}

	uri, ok := textDoc["uri"].(string)
	if !ok {
		return nil
	}

	// Only handle .gox files
	if !strings.HasSuffix(uri, ".gox") {
		return nil // Let gopls handle non-.gox files
	}

	p.log.Printf("Handling codeAction for %s (returning empty)", uri)

	// Return empty array - no code actions available yet
	return p.makeSuccessResponse(id, []any{})
}
