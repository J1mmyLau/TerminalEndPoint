# Quick Start Guide

Get TerminalEndPoint running in 5 minutes.

## Prerequisites

- Go 1.22+ (to build from source)
- Or Docker (to run containerized)
- macOS or Linux (PTY support required)

## 1. Build

```bash
git clone git@github.com:J1mmyLau/TerminalEndPoint.git
cd TerminalEndPoint
make build
```

Binary is at `./terminal-endpoint` (8.7MB).

## 2. Start (HTTP mode)

```bash
./terminal-endpoint
```

Output:
```
INFO starting terminal endpoint host=127.0.0.1 port=8080
INFO listening addr=127.0.0.1:8080
```

## 3. Health Check

```bash
curl http://localhost:8080/health
# {"status":"ok","sessions":0}
```

## 4. Execute a Command

```bash
curl -X POST http://localhost:8080/api/v1/sessions/none/exec \
  -H "Content-Type: application/json" \
  -d '{"command":"echo hello world && ls /tmp"}'
```

Response:
```json
{
  "exit_code": 0,
  "stdout": "hello world\nfile1\nfile2\n",
  "stderr": "",
  "duration_ms": 51
}
```

## 5. Interactive Session (WebSocket)

Create a session:
```bash
curl -X POST http://localhost:8080/api/v1/sessions \
  -H "Content-Type: application/json" \
  -d '{"label":"my-shell"}'
# Returns: {"id":"abc123...","status":"running",...}
```

Connect via WebSocket:
```bash
# Using websocat or similar tool:
websocat ws://localhost:8080/ws/sessions/abc123
```

Send commands and receive output in real-time.

## 6. MCP Mode (for AI Agents)

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | ./terminal-endpoint mcp
```

Configure your MCP client (Claude Desktop, Codex, etc.):
```json
{
  "mcpServers": {
    "terminal": {
      "command": "/path/to/terminal-endpoint",
      "args": ["mcp"]
    }
  }
}
```

## 7. Docker

```bash
docker compose up -d
curl http://localhost:8080/health
```

## Next Steps

- [API Reference](api.md) — full endpoint documentation
- [SKILL.md](../SKILL.md) — agent usage patterns (compile→debug→fix, gdb, REPL)
- Run integration tests: `python3 ../test_integration.py`
