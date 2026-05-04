# 快速入门指南

5 分钟上手 TerminalEndPoint。

## 环境要求

- Go 1.22+（从源码编译）
- 或 Docker（容器化运行）
- macOS 或 Linux（需要 PTY 支持）

## 1. 编译

```bash
git clone git@github.com:J1mmyLau/TerminalEndPoint.git
cd TerminalEndPoint
make build
```

二进制文件：`./terminal-endpoint`（8.7MB）。

## 2. 启动（HTTP 模式）

```bash
./terminal-endpoint
```

输出：
```
INFO starting terminal endpoint host=127.0.0.1 port=8080
INFO listening addr=127.0.0.1:8080
```

## 3. 健康检查

```bash
curl http://localhost:8080/health
# {"status":"ok","sessions":0}
```

## 4. 执行命令

```bash
curl -X POST http://localhost:8080/api/v1/sessions/none/exec \
  -H "Content-Type: application/json" \
  -d '{"command":"echo 你好世界 && ls /tmp"}'
```

返回：
```json
{
  "exit_code": 0,
  "stdout": "你好世界\nfile1\nfile2\n",
  "stderr": "",
  "duration_ms": 51
}
```

## 5. 交互式会话（WebSocket）

创建会话：
```bash
curl -X POST http://localhost:8080/api/v1/sessions \
  -H "Content-Type: application/json" \
  -d '{"label":"my-shell"}'
# 返回: {"id":"abc123...","status":"running",...}
```

通过 WebSocket 连接：
```bash
# 使用 websocat 或类似工具：
websocat ws://localhost:8080/ws/sessions/abc123
```

实时发送命令并接收输出。

## 6. MCP 模式（供 AI Agent 使用）

```bash
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | ./terminal-endpoint mcp
```

在 MCP 客户端中配置（Claude Desktop、Codex 等）：
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

## 下一步

- [API 参考](api_zh.md) — 完整端点文档
- [SKILL.md](../SKILL.md) — Agent 使用模式（编译→调试→修复、gdb、REPL）
- 运行集成测试：`python3 ../test_integration.py`
