package mcp

type Tool struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	InputSchema JSONSchema `json:"inputSchema"`
}

type JSONSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

type Property struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type ListToolsResult struct {
	Tools []Tool `json:"tools"`
}

type CallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type CallToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
	Data string `json:"data,omitempty"`
}

type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Capabilities    Capabilities `json:"capabilities"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
}

type Capabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

func (s *Server) BuildToolsList() []Tool {
	return []Tool{
		{
			Name:        "terminal_exec",
			Description: "Execute a command in a new terminal session and return the output. Creates a temporary session that runs the command and exits. Returns stdout, stderr, exit code, and duration.",
			InputSchema: JSONSchema{
				Type: "object",
				Properties: map[string]Property{
					"command": {Type: "string", Description: "The command to execute"},
					"timeout": {Type: "integer", Description: "Timeout in seconds (default: 30)"},
					"work_dir": {Type: "string", Description: "Working directory for the command"},
					"env": {Type: "object", Description: "Additional environment variables"},
				},
				Required: []string{"command"},
			},
		},
		{
			Name:        "terminal_spawn",
			Description: "Create an interactive terminal session. Returns a session ID that can be used with terminal_write, terminal_read, terminal_signal, terminal_resize, and terminal_kill.",
			InputSchema: JSONSchema{
				Type: "object",
				Properties: map[string]Property{
					"work_dir": {Type: "string", Description: "Working directory for the shell"},
					"shell":   {Type: "string", Description: "Shell to use (default: /bin/bash)"},
					"label":   {Type: "string", Description: "A human-readable label for this session"},
					"cols":    {Type: "integer", Description: "Terminal columns (default: 80)"},
					"rows":    {Type: "integer", Description: "Terminal rows (default: 24)"},
				},
			},
		},
		{
			Name:        "terminal_write",
			Description: "Write input to an interactive terminal session.",
			InputSchema: JSONSchema{
				Type: "object",
				Properties: map[string]Property{
					"session_id": {Type: "string", Description: "Session ID from terminal_spawn"},
					"data":      {Type: "string", Description: "Data to write (usually ends with newline)"},
				},
				Required: []string{"session_id", "data"},
			},
		},
		{
			Name:        "terminal_read",
			Description: "Read buffered output from a terminal session. Supports pagination with since and limit parameters.",
			InputSchema: JSONSchema{
				Type: "object",
				Properties: map[string]Property{
					"session_id": {Type: "string", Description: "Session ID from terminal_spawn"},
					"since":     {Type: "integer", Description: "Return entries with seq >= this value"},
					"limit":     {Type: "integer", Description: "Maximum number of entries to return"},
				},
				Required: []string{"session_id"},
			},
		},
		{
			Name:        "terminal_signal",
			Description: "Send a signal to a terminal session (SIGINT, SIGTERM, SIGKILL, SIGHUP).",
			InputSchema: JSONSchema{
				Type: "object",
				Properties: map[string]Property{
					"session_id": {Type: "string", Description: "Session ID from terminal_spawn"},
					"signal":    {Type: "string", Description: "Signal name: SIGINT, SIGTERM, SIGKILL, SIGHUP, SIGQUIT"},
				},
				Required: []string{"session_id", "signal"},
			},
		},
		{
			Name:        "terminal_resize",
			Description: "Resize a terminal session's dimensions.",
			InputSchema: JSONSchema{
				Type: "object",
				Properties: map[string]Property{
					"session_id": {Type: "string", Description: "Session ID from terminal_spawn"},
					"cols":      {Type: "integer", Description: "Number of columns"},
					"rows":      {Type: "integer", Description: "Number of rows"},
				},
				Required: []string{"session_id", "cols", "rows"},
			},
		},
		{
			Name:        "terminal_kill",
			Description: "Kill and clean up a terminal session.",
			InputSchema: JSONSchema{
				Type: "object",
				Properties: map[string]Property{
					"session_id": {Type: "string", Description: "Session ID from terminal_spawn"},
				},
				Required: []string{"session_id"},
			},
		},
		{
			Name:        "terminal_list",
			Description: "List all active terminal sessions with their status.",
			InputSchema: JSONSchema{
				Type:       "object",
				Properties: map[string]Property{},
			},
		},
		{
			Name:        "terminal_info",
			Description: "Get detailed information about a specific terminal session.",
			InputSchema: JSONSchema{
				Type: "object",
				Properties: map[string]Property{
					"session_id": {Type: "string", Description: "Session ID from terminal_spawn"},
				},
				Required: []string{"session_id"},
			},
		},
	}
}
