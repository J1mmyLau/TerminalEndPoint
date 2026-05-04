# TerminalEndPoint — Agent Tool Skill

**Trigger**: Agent needs to execute shell commands, run compilers/debuggers, manage long-running processes, or interact with a real terminal.

## Overview

TerminalEndPoint is a terminal control service that provides AI agents with full terminal access. It supports both MCP (Model Context Protocol) for local agent integration and HTTP/WebSocket API for remote access.

### Architecture

```
Agent ── MCP (stdio JSON-RPC) ──┐
Agent ── REST (HTTP)         ──┼── TerminalEndPoint Server
Agent ── WebSocket (stream)  ──┘       │
                                   PTY Sessions (bash/sh)
```

Each session is an isolated pseudo-terminal (PTY) with its own shell process. Sessions can be interactive (long-lived bash) or ephemeral (single command execution).

---

## MCP Tools Reference

The following tools are available via MCP. Each tool has a defined JSON schema.

### `terminal_exec` — Execute a command (non-interactive)

Execute a command in an ephemeral session. The command runs to completion and returns stdout, exit code, and duration.

```
Arguments:
  command  (required)  The shell command to execute
  timeout  (optional)  Timeout in seconds (default: 30)
  work_dir (optional)  Working directory
  env      (optional)  Environment variables map

Returns:
  Content: "Exit code: 0\nDuration: 50ms\n\n<output>"
```

**Example**:
```json
{
  "name": "terminal_exec",
  "arguments": {
    "command": "gcc -o prog prog.c && ./prog",
    "timeout": 30,
    "work_dir": "/tmp/build",
    "env": {"CFLAGS": "-O2"}
  }
}
```

**When to use**: Compilation, file operations, package installation, data processing, any command with a definite end.

**When NOT to use**: Interactive REPLs, long-running servers, programs that require user input mid-execution.

---

### `terminal_spawn` — Create an interactive session

Creates a long-lived bash session. Returns a session ID used with other tools.

```
Arguments:
  work_dir (optional) Working directory
  shell    (optional) Shell path (default: /bin/bash)
  label    (optional) Human-readable label
  cols     (optional) Terminal columns (default: 80)
  rows     (optional) Terminal rows (default: 24)

Returns:
  Content: "Session created: <uuid>\nStatus: running\nShell: /bin/bash"
```

**Example**:
```json
{
  "name": "terminal_spawn",
  "arguments": {
    "work_dir": "/project",
    "label": "debug-session"
  }
}
```

**When to use**: Interactive debugging (gdb/lldb), Python/Node REPL, any session where you need to send input and read output iteratively.

---

### `terminal_write` — Send input to a session

Write data (usually a command ending with `\n`) to an interactive session.

```
Arguments:
  session_id (required) Session ID from terminal_spawn
  data       (required) Data to write (include newline for commands)

Returns:
  Content: "Written"
```

**Example**:
```json
{
  "name": "terminal_write",
  "arguments": {
    "session_id": "abc123",
    "data": "ls -la\n"
  }
}
```

**Important**: Always end commands with `\n` (newline) to execute them in the shell.

---

### `terminal_read` — Read buffered output

Read output from a session's ring buffer. Supports pagination.

```
Arguments:
  session_id (required) Session ID
  since      (optional) Return entries with seq >= this value
  limit      (optional) Max entries to return

Returns:
  Content: buffered output text
```

**Example**:
```json
{
  "name": "terminal_read",
  "arguments": {
    "session_id": "abc123",
    "since": 5,
    "limit": 20
  }
}
```

**Pattern**: After `terminal_write`, use `terminal_read` to check for new output.

---

### `terminal_signal` — Send a signal

Send a POSIX signal to the session's process.

```
Arguments:
  session_id (required) Session ID
  signal     (required) One of: SIGINT, SIGTERM, SIGKILL, SIGHUP, SIGQUIT

Returns:
  Content: "Signal SIGINT sent to session abc123"
```

**Common patterns**:
- `SIGINT` — Interrupt a running command (Ctrl-C equivalent)
- `SIGTERM` — Graceful termination
- `SIGKILL` — Force kill

---

### `terminal_resize` — Resize terminal

Change terminal dimensions (affects line-wrapping in some programs).

```
Arguments:
  session_id (required) Session ID
  cols       (required) Columns
  rows       (required) Rows
```

---

### `terminal_kill` — Terminate a session

Kill the session process and free resources.

```
Arguments:
  session_id (required) Session ID
```

**Always call this** when done with a session to prevent resource leaks.

---

### `terminal_list` — List active sessions

```
Arguments: (none)

Returns:
  Content: table of session IDs, statuses, shells, labels
```

---

### `terminal_info` — Get session details

```
Arguments:
  session_id (required) Session ID

Returns:
  Content: ID, Status, Shell, WorkDir, ExitCode, Label, timestamps
```

---

## Agent Workflow Patterns

### Pattern 1: Compile → Run → Fix (Non-interactive)

For code generation + verification loops:

```
1. terminal_exec: write source file    → echo "code..." > file.c
2. terminal_exec: compile              → gcc -o prog file.c 2>&1
3. terminal_exec: run                  → ./prog
4. If exit_code != 0: analyze output, fix, goto 1
5. terminal_exec: test                 → ./prog < test_input
```

**Key**: Use `terminal_exec` for each step. It's stateless and simple.

### Pattern 2: Interactive Debug (GDB/LLDB)

For debugging where you need to set breakpoints, inspect variables, step through code:

```
1. terminal_spawn:  create session          → label="gdb-debug"
2. terminal_write:  compile with -g flag    → gcc -g -O0 -o prog prog.c\n
3. terminal_read:   verify compile output
4. terminal_write:  start debugger          → gdb -q ./prog\n
5. terminal_read:   wait for (gdb) prompt
6. terminal_write:  set breakpoint          → break add\n
7. terminal_write:  run program             → run\n
8. terminal_read:   inspect output (hit breakpoint?)
9. terminal_write:  print variable          → print my_var\n
10. terminal_read:  get variable value
11. terminal_write: continue/step/next      → continue\n
12. terminal_kill:  clean up
```

**Key**: Always `terminal_read` between writes to verify the tool's state.

### Pattern 3: Long-running Build/Process

For commands that take minutes (compilation, training, deployment):

```
1. terminal_exec: start with long timeout
   → command: "make build", timeout: 600
2. Check result: exit_code, stdout, duration_ms, truncated
   → If truncated: output exceeded buffer, check tail of stdout
```

For progressive output monitoring (WebSocket mode):
```
1. terminal_spawn:  create build session
2. WS connect:      ws://host/ws/sessions/{id}
3. terminal_write:  make build\n
4. WS stream:       receive real-time output events
5. Wait for exit event
```

### Pattern 4: REPL (Python, Node, etc.)

```
1. terminal_spawn:  create session → shell="/bin/bash"
2. terminal_write:  python3\n
3. terminal_read:   wait for >>> prompt
4. terminal_write:  print(1+2)\n
5. terminal_read:   get result (3)
6. terminal_write:  import json\njson.dumps({"key":"val"})\n
7. terminal_read:   get JSON output
8. terminal_kill:   clean up
```

### Pattern 5: File Creation + Execution

Creating files from agent-generated content:

```bash
# Method 1: heredoc (watch for shell escaping of \n, $, etc.)
cat > script.py << 'PYEOF'
import sys
print(sys.argv)
PYEOF

# Method 2: Multi-line echo (safer for simple files)
echo 'print(1+1)' > script.py

# Method 3: Write via Python (most reliable for complex content)
python3 -c "
with open('file.c', 'w') as f:
    f.write('''#include <stdio.h>
int main() { return 0; }
''')
"
```

---

## Error Handling

### Check exit code
```
Always verify terminal_exec exit_code == 0 for success.
Non-zero means: compile error, runtime crash, command not found.
```

### Truncated output
```
If truncated == true: output exceeded the buffer limit (default 100KB).
For non-interactive commands, increase timeout and re-run.
For long output, use terminal_read with pagination.
```

### Timeout
```
If command doesn't complete within timeout:
- exit_code will be -1 (still running)
- truncated will be true
- stdout contains output produced before timeout
```

### Session not found
```
If terminal_write/read returns "session not found":
- Session was killed (manually or by TTL)
- Session TTL is 5 minutes of inactivity by default
- Re-create with terminal_spawn
```

---

## Best Practices

1. **Prefer `terminal_exec` over `terminal_spawn`** for deterministic tasks. It's simpler, faster, and self-cleaning.

2. **Always include `2>&1`** in compile/test commands to capture stderr in stdout.

3. **Set appropriate timeouts**: 30s for quick commands, 120s for compilation, 600s for heavy builds.

4. **Use `work_dir`** to set context rather than `cd` commands — more reliable.

5. **Clean up interactive sessions** with `terminal_kill` when done. Sessions auto-expire after 5 minutes of inactivity, but explicit cleanup is better.

6. **Shell escaping**: Commands are executed via `/bin/sh -c`. Be careful with special characters (`$`, `\`, `"`, `` ` ``). Use single quotes or Python one-liners to avoid escaping issues.

7. **For binary output**: Output is delivered as text (UTF-8). Binary data in terminal output will be garbled.

8. **Concurrent sessions**: You can have up to 50 concurrent sessions. Use `terminal_list` to track them.

---

## HTTP/WebSocket API (Alternative to MCP)

The same functionality is available via HTTP REST and WebSocket for remote agents.

### REST Endpoints
```
POST   /api/v1/sessions              Create interactive session
GET    /api/v1/sessions              List sessions
GET    /api/v1/sessions/{id}         Get session info
DELETE /api/v1/sessions/{id}         Kill session

POST   /api/v1/sessions/{id}/exec   Execute command (non-interactive)
POST   /api/v1/sessions/{id}/write  Write input
GET    /api/v1/sessions/{id}/output Read output (?since=N&limit=M)
POST   /api/v1/sessions/{id}/resize Resize terminal
POST   /api/v1/sessions/{id}/signal Send signal
```

### WebSocket
```
ws://host/ws/sessions/{id}          Stream output, send input
ws://host/ws/sessions/{id}?since_seq=42  Reconnect with history replay
```

### Quick start (HTTP)
```bash
# Start server
./terminal-endpoint

# Execute a command
curl -X POST http://localhost:8080/api/v1/sessions/none/exec \
  -H "Content-Type: application/json" \
  -d '{"command":"echo hello && uname -a"}'
```

### Quick start (MCP)
```bash
# Start MCP server
./terminal-endpoint mcp

# Send JSON-RPC requests via stdin
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' | ./terminal-endpoint mcp
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HOST` | 127.0.0.1 | Listen address |
| `PORT` | 8080 | Listen port |
| `MAX_SESSIONS` | 50 | Max concurrent sessions |
| `SESSION_TTL` | 5m | Idle session timeout |
| `MAX_EXEC_TIMEOUT` | 120s | Default exec timeout |
| `MAX_OUTPUT_LINES` | 1000 | Ring buffer capacity |
