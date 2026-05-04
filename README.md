# TerminalEndPoint

Terminal control endpoint for AI agents. Provides a complete terminal manipulation layer with dual-mode access (MCP + HTTP/WebSocket), streaming I/O, and full PTY lifecycle management.

## Why

AI agents need to run shell commands, compile code, debug programs, and interact with REPLs. TerminalEndPoint wraps these capabilities into a clean API that agents can call — whether locally via MCP's JSON-RPC over stdio, or remotely via REST + WebSocket.

## Quick Start

```bash
# HTTP mode (default)
./terminal-endpoint
# Starts at http://localhost:8080

# MCP mode (for agent integration)
./terminal-endpoint mcp
```

Test it:
```bash
curl -X POST http://localhost:8080/api/v1/sessions/none/exec \
  -H "Content-Type: application/json" \
  -d '{"command":"echo hello && uname -a"}'
```

## Project Structure

```
TerminalEndPoint/
├── cmd/terminal-endpoint/
│   ├── main.go              # Entry point: HTTP (default) or MCP mode
│   └── mcp.go               # MCP stdio JSON-RPC server
├── internal/
│   ├── config/               # Environment-based configuration
│   ├── pty/                  # PTY wrapper (creack/pty)
│   ├── session/              # Session lifecycle, ring buffer, event bus, terminal responder
│   ├── server/               # HTTP server, router, CORS middleware
│   ├── handler/              # REST + WebSocket handlers
│   └── mcp/                  # MCP JSON-RPC server + 9 tool definitions
├── pkg/protocol/             # Shared API types
├── docs/                     # Documentation
│   ├── quickstart.md         # 5-minute start
│   └── api.md                # Full API reference
├── SKILL.md                  # Agent usage guide (MCP tools + patterns)
├── Dockerfile                # Multi-stage build → 12MB Alpine
├── docker-compose.yml        # One-command deployment
└── Makefile
```

## Features

- **Dual-mode access**: MCP (stdin/stdout JSON-RPC) for local agents, HTTP/WebSocket for remote
- **Streaming I/O**: Real-time bidirectional terminal I/O over WebSocket
- **PTY-backed sessions**: Full pseudo-terminal support (resize, signals, raw output)
- **Ring buffer**: Bounded output history with sequence-based replay on reconnect
- **Session management**: TTL-based auto-cleanup, concurrent session pool
- **TUI tool support**: Built-in terminal query responder (VT100/xterm) for vim, htop, codex TUI, REPLs
- **Interactive REPL**: Full bash/python/node REPL support with write/read cycle
- **9 MCP tools**: exec, spawn, write, read, signal, resize, kill, list, info
- **Race-free**: All concurrency primitives verified with Go race detector
- **Single binary**: 8.7MB, no runtime dependencies (linux/amd64, darwin/arm64)

## API at a Glance

### REST

```
POST   /api/v1/sessions              Create interactive terminal session
GET    /api/v1/sessions              List active sessions
GET    /api/v1/sessions/:id          Get session details
DELETE /api/v1/sessions/:id          Kill and clean up session

POST   /api/v1/sessions/{id}/exec   Execute command (ephemeral session)
POST   /api/v1/sessions/{id}/write  Write input to session
GET    /api/v1/sessions/{id}/output Read output (?since=N&limit=M)
POST   /api/v1/sessions/{id}/resize Resize terminal (cols x rows)
POST   /api/v1/sessions/{id}/signal Send signal (SIGINT|SIGTERM|SIGKILL|SIGHUP)
```

### WebSocket

```
ws://host/ws/sessions/:id             Stream real-time output + send input
ws://host/ws/sessions/:id?since_seq=N Reconnect with output replay
```

### MCP Tools

| Tool | Description |
|------|-------------|
| `terminal_exec` | Execute command, return exit code + stdout + duration |
| `terminal_spawn` | Create interactive session, return session ID |
| `terminal_write` | Write input to session |
| `terminal_read` | Read buffered output (paginated) |
| `terminal_signal` | Send POSIX signal |
| `terminal_resize` | Change terminal dimensions |
| `terminal_kill` | Terminate and clean up session |
| `terminal_list` | List all active sessions |
| `terminal_info` | Get session details |

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `HOST` | 127.0.0.1 | Listen address |
| `PORT` | 8080 | Listen port |
| `MAX_SESSIONS` | 50 | Max concurrent sessions |
| `SESSION_TTL` | 5m | Idle session timeout |
| `MAX_EXEC_TIMEOUT` | 120s | Default exec command timeout |
| `MAX_OUTPUT_LINES` | 1000 | Ring buffer line capacity |

## Docker

```bash
docker compose up -d
curl http://localhost:8080/health
```

## Development

```bash
make build      # Build binary
make run        # Build + run
make test       # Run Go tests
make vet        # Static analysis
make fmt        # Format code

# Run with race detector
go test -race ./...

# Integration tests (requires running server)
./terminal-endpoint &
python3 test_integration.py
python3 test_agent_workflow.py
```

## Documentation

- [Quick Start Guide](docs/quickstart.md) — 5-minute walkthrough
- [API Reference](docs/api.md) — Full endpoint + MCP tool reference
- [SKILL.md](SKILL.md) — Agent tool usage guide with workflow patterns
- [Architecture Plan](.sisyphus/plans/architecture.md) — Design decisions and data flow

## License

MIT
