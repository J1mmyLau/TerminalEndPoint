package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"
)

func TestServer_RequestResponse(t *testing.T) {
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n")
	output := &bytes.Buffer{}

	server := NewServer(input, output)

	server.Register("initialize", func(params json.RawMessage) (interface{}, error) {
		return map[string]string{"status": "ok"}, nil
	})

	server.Serve()

	var resp Response
	if err := json.NewDecoder(output).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID != float64(1) {
		t.Fatalf("expected id 1, got %v", resp.ID)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected result")
	}
}

func TestServer_MethodNotFound(t *testing.T) {
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":2,"method":"unknown_method","params":{}}` + "\n")
	output := &bytes.Buffer{}

	server := NewServer(input, output)

	server.Register("initialize", func(params json.RawMessage) (interface{}, error) {
		return map[string]string{}, nil
	})

	server.Serve()

	var resp Response
	if err := json.NewDecoder(output).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Fatalf("expected code -32601, got %d", resp.Error.Code)
	}
}

func TestServer_Notification(t *testing.T) {
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}` + "\n")
	output := &bytes.Buffer{}

	server := NewServer(input, output)

	notified := false
	server.Register("notifications/initialized", func(params json.RawMessage) (interface{}, error) {
		notified = true
		return nil, nil
	})

	server.Serve()

	if !notified {
		t.Fatal("notification handler was not called")
	}
	if output.Len() > 0 {
		t.Fatal("notification should not produce a response")
	}
}

func TestServer_HandlerError(t *testing.T) {
	input := bytes.NewBufferString(`{"jsonrpc":"2.0","id":3,"method":"failing","params":{}}` + "\n")
	output := &bytes.Buffer{}

	server := NewServer(input, output)
	server.Register("failing", func(params json.RawMessage) (interface{}, error) {
		return nil, fmt.Errorf("handler error")
	})

	server.Serve()

	var resp Response
	if err := json.NewDecoder(output).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error from failing handler")
	}
	if resp.Error.Code != -32000 {
		t.Fatalf("expected code -32000, got %d", resp.Error.Code)
	}
}

func TestServer_InvalidJSON(t *testing.T) {
	input := bytes.NewBufferString(`not json` + "\n")
	output := &bytes.Buffer{}

	server := NewServer(input, output)
	server.Serve()

	if output.Len() > 0 {
		t.Fatal("invalid JSON should not produce output")
	}
}

func TestCallToolResult(t *testing.T) {
	result := CallToolResult{
		Content: []ContentItem{
			{Type: "text", Text: "hello"},
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var unmarshaled CallToolResult
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(unmarshaled.Content) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(unmarshaled.Content))
	}
	if unmarshaled.Content[0].Type != "text" {
		t.Fatalf("expected type 'text', got '%s'", unmarshaled.Content[0].Type)
	}
}

func TestToolDefinitions(t *testing.T) {
	server := NewServer(nil, nil)
	tools := server.BuildToolsList()

	if len(tools) != 9 {
		t.Fatalf("expected 9 tools, got %d", len(tools))
	}

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Name] = true
		if tool.Description == "" {
			t.Fatalf("tool %s has no description", tool.Name)
		}
	}

	expected := []string{
		"terminal_exec", "terminal_spawn", "terminal_write",
		"terminal_read", "terminal_signal", "terminal_resize",
		"terminal_kill", "terminal_list", "terminal_info",
	}
	for _, name := range expected {
		if !names[name] {
			t.Fatalf("missing tool: %s", name)
		}
	}
}

func TestInitializeResult(t *testing.T) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: Capabilities{
			Tools: &ToolsCapability{ListChanged: false},
		},
		ServerInfo: ServerInfo{
			Name:    "terminal-endpoint",
			Version: "0.1.0",
		},
	}

	data, _ := json.Marshal(result)

	var m map[string]interface{}
	json.Unmarshal(data, &m)

	if m["protocolVersion"] != "2024-11-05" {
		t.Fatalf("protocol version mismatch")
	}
}
