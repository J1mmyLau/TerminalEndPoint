# API Reference

TerminalEndPoint exposes two interfaces: a REST + WebSocket HTTP API, and an MCP (Model Context Protocol) JSON-RPC interface over stdio.

> **TUI Support**: TerminalEndPoint includes a built-in terminal query responder that auto-answers VT100/xterm device queries (cursor position, device attributes, color queries). This enables full interaction with TUI applications like vim, htop, codex, and REPLs without the terminal hanging.

## HTTP REST API

Base URL: `http://host:port`

All request/response bodies are JSON. Timestamps are RFC 3339.

### Sessions

#### Create Session

```
POST /api/v1/sessions
```

**Request:**
```json
{
  "label": "my-session",
  "work_dir": "/home/user/project",
  "shell": "/bin/bash",
  "cols": 120,
  "rows": 40,
  "env": {
    "MY_VAR": "value"
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| label | string | "" | Human-readable name |
| work_dir | string | "" | Working directory |
| shell | string | "/bin/bash" | Shell path |
| cols | uint16 | 80 | Terminal columns |
| rows | uint16 | 24 | Terminal rows |
| env | map | {} | Additional env vars (merged with server env) |

**Response:** `201 Created`
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "label": "my-session",
  "work_dir": "/home/user/project",
  "shell": "/bin/bash",
  "status": "running",
  "exit_code": -1,
  "created_at": "2026-05-04T12:00:00Z",
  "updated_at": "2026-05-04T12:00:00Z"
}
```

Status values: `running`, `exited`, `killed`, `error`. Exit code is -1 while running.

---

#### List Sessions

```
GET /api/v1/sessions
```

**Response:** `200 OK`
```json
{
  "sessions": [
    {
      "id": "550e8400-...",
      "label": "my-session",
      "status": "running",
      ...
    }
  ],
  "count": 1
}
```

---

#### Get Session

```
GET /api/v1/sessions/{id}
```

**Response:** `200 OK` — same shape as Create Session response.

**Errors:** `404` — session not found.

---

#### Kill Session

```
DELETE /api/v1/sessions/{id}
```

**Response:** `200 OK`
```json
{"status": "killed", "id": "550e8400-..."}
```

Sends SIGTERM, closes PTY, waits for exit, force-kills if needed.

---

### Command Execution

#### Execute Command

```
POST /api/v1/sessions/{id}/exec
```

The `{id}` path segment is ignored — each exec creates an ephemeral session that runs `/bin/sh -c <command>` and exits. The session is automatically cleaned up.

**Request:**
```json
{
  "command": "gcc -O2 -o prog prog.c && ./prog",
  "timeout_seconds": 30,
  "work_dir": "/tmp/build",
  "env": {
    "CFLAGS": "-Wall"
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| command | string | (required) | Shell command to execute |
| timeout_seconds | int | 120 | Max execution time |
| work_dir | string | "" | Working directory |
| env | map | {} | Additional environment variables |

**Response:** `200 OK`
```json
{
  "exit_code": 0,
  "stdout": "compilation output...\nprogram output...\n",
  "stderr": "",
  "duration_ms": 1523,
  "truncated": false
}
```

`truncated: true` indicates output exceeded the 100KB buffer limit. The returned `stdout` contains what was captured before truncation. `exit_code` is -1 if the command was killed by timeout.

**Errors:** `400` — missing command, invalid JSON. `500` — session creation failed.

---

### Session I/O

#### Write Input

```
POST /api/v1/sessions/{id}/write
```

**Request:**
```json
{
  "data": "ls -la\n"
}
```

Include `\n` to execute the command in the shell.

**Response:** `200 OK`
```json
{"status": "written"}
```

**Errors:** `404` — session not found or not running.

---

#### Read Output

```
GET /api/v1/sessions/{id}/output?since=0&limit=50
```

Query parameters:

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| since | uint64 | 0 | Return entries with seq >= this value |
| limit | int | 0 (all) | Max entries to return |

**Response:** `200 OK`
```json
{
  "entries": [
    {"seq": 1, "data": "SGVsbG8gd29ybGQ="},
    {"seq": 2, "data": "YW5vdGhlciBsaW5l"}
  ],
  "latest_seq": 2
}
```

`data` is base64-encoded output. Decode to get the raw bytes.

---

#### Resize Terminal

```
POST /api/v1/sessions/{id}/resize
```

**Request:**
```json
{
  "cols": 120,
  "rows": 40
}
```

**Response:** `200 OK`
```json
{"status": "resized"}
```

---

#### Send Signal

```
POST /api/v1/sessions/{id}/signal
```

**Request:**
```json
{
  "signal": "SIGINT"
}
```

Valid signals: `SIGINT`, `SIGTERM`, `SIGKILL`, `SIGHUP`, `SIGQUIT`.

**Response:** `200 OK`
```json
{"status": "signaled", "signal": "SIGINT"}
```

---

### Health

```
GET /health
```

**Response:** `200 OK`
```json
{"status": "ok", "sessions": 3}
```

---

## WebSocket API

### Connect

```
ws://host/ws/sessions/{id}
ws://host/ws/sessions/{id}?since_seq=42
```

On connect with `since_seq`, the server replays buffered output entries with seq >= the given value, then enters real-time streaming mode.

### Client → Server Messages

```json
{"type": "write", "data": {"data": "ls -la\n"}}
{"type": "resize", "data": {"cols": 120, "rows": 40}}
{"type": "signal", "data": {"signal": "SIGINT"}}
```

### Server → Client Messages

**History replay (on connect with since_seq):**
```json
{
  "type": "history",
  "data": {
    "entries": [
      {"stream": "stdout", "data": "SGVsbG8=", "seq": 1},
      {"stream": "stdout", "data": "d29ybGQ=", "seq": 2}
    ],
    "next_seq": 3
  }
}
```

**Real-time output:**
```json
{
  "type": "output",
  "data": {
    "stream": "stdout",
    "data": "c29tZSBvdXRwdXQ=",
    "seq": 3
  }
}
```

**Process exit:**
```json
{
  "type": "exit",
  "data": {"code": 0}
}
```

**Error:**
```json
{
  "type": "error",
  "data": {"message": "session not found"}
}
```

All `data` fields in output messages are base64 encoded.

### Ping/Pong

The server sends WebSocket ping frames every 30 seconds. Clients should respond with pong frames.

---

## MCP Protocol (JSON-RPC over stdio)

Start MCP mode: `./terminal-endpoint mcp`

The server reads JSON-RPC requests from stdin (one per line, newline-delimited) and writes responses to stdout.

### Lifecycle

**Initialize:**
```json
// → Request
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}

// ← Response
{
  "jsonrpc":"2.0","id":1,
  "result":{
    "protocolVersion":"2024-11-05",
    "capabilities":{"tools":{"listChanged":false}},
    "serverInfo":{"name":"terminal-endpoint","version":"0.1.0"}
  }
}
```

### Tool Discovery

**List tools:**
```json
// → Request
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}

// ← Response
{"jsonrpc":"2.0","id":2,"result":{"tools":[...]}}
```

### Tool Invocation

**Call a tool:**
```json
{
  "jsonrpc":"2.0",
  "id":3,
  "method":"tools/call",
  "params":{
    "name":"terminal_exec",
    "arguments":{
      "command":"echo hello",
      "timeout": 10
    }
  }
}
```

**Response:**
```json
{
  "jsonrpc":"2.0",
  "id":3,
  "result":{
    "content":[
      {"type":"text","text":"Exit code: 0\nDuration: 50ms\n\nhello\n"}
    ]
  }
}
```

### Tool Reference

See [SKILL.md](../SKILL.md) for tool-by-tool documentation with examples.

### Errors

```json
{
  "jsonrpc":"2.0",
  "id":4,
  "error":{
    "code":-32601,
    "message":"method not found: unknown_method"
  }
}
```

Error codes:
| Code | Meaning |
|------|---------|
| -32601 | Method not found |
| -32000 | Tool execution error (message contains details) |

---

## Error Response Format

All REST errors follow this shape:
```json
{
  "error": "human-readable message",
  "code": 400
}
```

HTTP status codes:
| Code | Meaning |
|------|---------|
| 200 | Success |
| 201 | Session created |
| 400 | Bad request (invalid JSON, missing fields) |
| 404 | Session not found |
| 500 | Internal error (PTY failure, max sessions reached) |

---

## Session Lifecycle

```
CREATED ──exec──▶ runs command, exits, cleaned up
CREATED ──spawn─▶ RUNNING ──write/read──▶ (interactive)
RUNNING ──signal─▶ EXITED (or KILLED)
EXITED ──TTL(5m)─▶ cleaned up
RUNNING ──DELETE─▶ KILLED → cleaned up
```

Sessions are automatically cleaned up after 5 minutes of inactivity (configurable via `SESSION_TTL`).
