package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"strings"
	"testing"

	"github.com/germtb/gox/generator"
)

// testProxy creates a Proxy suitable for testing (with a no-op logger)
func testProxy() *Proxy {
	return &Proxy{
		sourceMaps:   make(map[string]*generator.SourceMap),
		fileContents: make(map[string]string),
		log:          log.New(io.Discard, "", 0),
	}
}

func TestUriToPath(t *testing.T) {
	tests := []struct {
		uri      string
		expected string
	}{
		{"file:///path/to/file.gox", "/path/to/file.gox"},
		{"file:///Users/test/app.gox", "/Users/test/app.gox"},
		{"/already/a/path.gox", "/already/a/path.gox"},
	}

	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			result := uriToPath(tt.uri)
			if result != tt.expected {
				t.Errorf("uriToPath(%q) = %q, want %q", tt.uri, result, tt.expected)
			}
		})
	}
}

func TestPathToUri(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/path/to/file.gox", "file:///path/to/file.gox"},
		{"/Users/test/app.gox", "file:///Users/test/app.gox"},
		{"file:///already/uri.gox", "file:///already/uri.gox"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := pathToURI(tt.path)
			if result != tt.expected {
				t.Errorf("pathToURI(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestReadMessage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "valid message",
			input:    "Content-Length: 13\r\n\r\n{\"test\":true}",
			expected: `{"test":true}`,
			wantErr:  false,
		},
		{
			name:     "message with extra headers",
			input:    "Content-Length: 13\r\nContent-Type: application/json\r\n\r\n{\"test\":true}",
			expected: `{"test":true}`,
			wantErr:  false,
		},
		{
			name:    "missing content length",
			input:   "\r\n{\"test\":true}",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))
			result, err := readMessage(reader)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if string(result) != tt.expected {
				t.Errorf("readMessage() = %q, want %q", string(result), tt.expected)
			}
		})
	}
}

func TestWriteMessage(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"result":null}`)

	var buf bytes.Buffer
	err := writeMessage(&buf, body)
	if err != nil {
		t.Fatalf("writeMessage error: %v", err)
	}

	result := buf.String()
	expectedHeader := "Content-Length: 38\r\n\r\n"
	if !strings.HasPrefix(result, expectedHeader) {
		t.Errorf("Expected header %q, got prefix %q", expectedHeader, result[:len(expectedHeader)])
	}

	if !strings.HasSuffix(result, string(body)) {
		t.Errorf("Expected body %q at end", string(body))
	}
}

func TestMakeSuccessResponse(t *testing.T) {
	p := testProxy()

	result := p.makeSuccessResponse(1, map[string]any{"foo": "bar"})

	var response map[string]any
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc 2.0, got %v", response["jsonrpc"])
	}

	if response["id"] != float64(1) {
		t.Errorf("Expected id 1, got %v", response["id"])
	}

	resultObj, ok := response["result"].(map[string]any)
	if !ok {
		t.Fatal("Expected result to be map")
	}

	if resultObj["foo"] != "bar" {
		t.Errorf("Expected foo=bar, got %v", resultObj["foo"])
	}
}

func TestMakeErrorResponse(t *testing.T) {
	p := testProxy()

	result := p.makeErrorResponse(1, -32600, "Invalid Request")

	var response map[string]any
	if err := json.Unmarshal(result, &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc 2.0, got %v", response["jsonrpc"])
	}

	if response["id"] != float64(1) {
		t.Errorf("Expected id 1, got %v", response["id"])
	}

	errObj, ok := response["error"].(map[string]any)
	if !ok {
		t.Fatal("Expected error to be map")
	}

	if errObj["code"] != float64(-32600) {
		t.Errorf("Expected code -32600, got %v", errObj["code"])
	}

	if errObj["message"] != "Invalid Request" {
		t.Errorf("Expected message 'Invalid Request', got %v", errObj["message"])
	}
}

func TestGoxToGoPath(t *testing.T) {
	p := testProxy()

	tests := []struct {
		goxPath  string
		expected string
	}{
		{"/path/to/app.gox", "/path/to/app_gox.go"},
		{"/Users/test/component.gox", "/Users/test/component_gox.go"},
		{"relative/path.gox", "relative/path_gox.go"},
	}

	for _, tt := range tests {
		t.Run(tt.goxPath, func(t *testing.T) {
			result := p.goxToGoPath(tt.goxPath)
			if result != tt.expected {
				t.Errorf("goxToGoPath(%q) = %q, want %q", tt.goxPath, result, tt.expected)
			}
		})
	}
}

func TestRewriteURIs(t *testing.T) {
	p := testProxy()

	t.Run("rewrites gox to go", func(t *testing.T) {
		obj := map[string]any{
			"textDocument": map[string]any{
				"uri": "file:///path/to/app.gox",
			},
		}

		p.rewriteURIs(obj, true)

		textDoc := obj["textDocument"].(map[string]any)
		if textDoc["uri"] != "file:///path/to/app_gox.go" {
			t.Errorf("Expected rewritten URI, got %v", textDoc["uri"])
		}
	})

	t.Run("preserves non-gox URIs", func(t *testing.T) {
		obj := map[string]any{
			"textDocument": map[string]any{
				"uri": "file:///path/to/regular.go",
			},
		}

		p.rewriteURIs(obj, true)

		textDoc := obj["textDocument"].(map[string]any)
		if textDoc["uri"] != "file:///path/to/regular.go" {
			t.Errorf("Expected unchanged URI, got %v", textDoc["uri"])
		}
	})

	t.Run("handles arrays", func(t *testing.T) {
		obj := map[string]any{
			"items": []any{
				map[string]any{"uri": "file:///a.gox"},
				map[string]any{"uri": "file:///b.gox"},
			},
		}

		p.rewriteURIs(obj, true)

		items := obj["items"].([]any)
		item1 := items[0].(map[string]any)
		item2 := items[1].(map[string]any)

		if item1["uri"] != "file:///a_gox.go" {
			t.Errorf("Expected rewritten URI for item 1, got %v", item1["uri"])
		}
		if item2["uri"] != "file:///b_gox.go" {
			t.Errorf("Expected rewritten URI for item 2, got %v", item2["uri"])
		}
	})
}

func TestHandleRequestDirectly(t *testing.T) {
	p := testProxy()

	t.Run("returns nil for non-handled methods", func(t *testing.T) {
		msg := []byte(`{"jsonrpc":"2.0","id":1,"method":"textDocument/hover","params":{}}`)
		result := p.handleRequestDirectly(msg)
		if result != nil {
			t.Errorf("Expected nil for unhandled method, got %s", result)
		}
	})

	t.Run("handles codeAction for gox files", func(t *testing.T) {
		msg := []byte(`{"jsonrpc":"2.0","id":1,"method":"textDocument/codeAction","params":{"textDocument":{"uri":"file:///test.gox"}}}`)
		result := p.handleRequestDirectly(msg)
		if result == nil {
			t.Fatal("Expected response for codeAction")
		}

		var response map[string]any
		if err := json.Unmarshal(result, &response); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		// Should return empty array
		resultArr, ok := response["result"].([]any)
		if !ok {
			t.Fatal("Expected result to be array")
		}
		if len(resultArr) != 0 {
			t.Errorf("Expected empty array, got %v", resultArr)
		}
	})

	t.Run("returns nil for codeAction on non-gox files", func(t *testing.T) {
		msg := []byte(`{"jsonrpc":"2.0","id":1,"method":"textDocument/codeAction","params":{"textDocument":{"uri":"file:///test.go"}}}`)
		result := p.handleRequestDirectly(msg)
		if result != nil {
			t.Errorf("Expected nil for non-gox codeAction, got %s", result)
		}
	})
}
