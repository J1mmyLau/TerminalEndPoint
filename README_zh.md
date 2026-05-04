# TerminalEndPoint

为 AI Agent 提供完善的终端控制端点，支持流式操控。提供双模式接入（MCP + HTTP/WebSocket）、流式 I/O 和完整的 PTY 生命周期管理。

## 为什么需要

AI Agent 需要执行 shell 命令、编译代码、调试程序、操作 REPL。TerminalEndPoint 将这些能力封装成干净的 API，Agent 可以通过 MCP 的 JSON-RPC over stdio 本地调用，也可以通过 REST + WebSocket 远程调用。

## 快速开始

```bash
# HTTP 模式（默认）
./terminal-endpoint
# 启动在 http://localhost:8080

# MCP 模式（供 Agent 集成）
./terminal-endpoint mcp
```

验证：
```bash
curl -X POST http://localhost:8080/api/v1/sessions/none/exec \
  -H "Content-Type: application/json" \
  -d '{"command":"echo hello && uname -a"}'
```

## 项目结构

```
TerminalEndPoint/
├── cmd/terminal-endpoint/
│   ├── main.go              # 入口：HTTP（默认）或 MCP 模式
│   └── mcp.go               # MCP stdio JSON-RPC 服务
├── internal/
│   ├── config/               # 环境变量配置
│   ├── pty/                  # PTY 封装（creack/pty）
│   ├── session/              # 会话生命周期、环形缓冲区、事件总线、终端应答器
│   ├── server/               # HTTP 服务器、路由、CORS 中间件
│   ├── handler/              # REST + WebSocket 处理器
│   └── mcp/                  # MCP JSON-RPC 服务 + 9 个工具定义
├── pkg/protocol/             # 共享 API 类型
├── docs/                     # 文档
│   ├── quickstart.md         # 5 分钟快速入门
│   ├── quickstart_zh.md      # 5 分钟快速入门（中文）
│   ├── api.md                # 完整 API 参考
│   └── api_zh.md             # 完整 API 参考（中文）
├── SKILL.md                  # Agent 使用指南（MCP 工具 + 工作流模式）
├── Dockerfile                # 多阶段构建 → 12MB Alpine
├── docker-compose.yml        # 一键部署
└── Makefile
```

## 核心特性

- **双模式接入**：MCP（stdin/stdout JSON-RPC）供本地 Agent + HTTP/WebSocket 供远程调用
- **流式 I/O**：WebSocket 实时双向终端数据传输
- **PTY 会话**：完整的伪终端支持（resize、信号、原始输出）
- **环形缓冲区**：有界输出历史，支持断线重连时按序号回放
- **会话管理**：TTL 自动回收，并发会话池
- **TUI 工具支持**：内置终端查询应答器（VT100/xterm），支持 vim、htop、codex TUI、REPL 等
- **交互式 REPL**：完整的 bash/python/node REPL 支持（write/read 循环）
- **9 个 MCP 工具**：exec、spawn、write、read、signal、resize、kill、list、info
- **并发安全**：Go race detector 验证零竞态
- **单二进制**：8.7MB，无运行时依赖（linux/amd64、darwin/arm64）

## API 一览

### REST

```
POST   /api/v1/sessions              创建交互式终端会话
GET    /api/v1/sessions              列出活跃会话
GET    /api/v1/sessions/:id          获取会话详情
DELETE /api/v1/sessions/:id          终止并清理会话

POST   /api/v1/sessions/{id}/exec   执行命令（临时会话）
POST   /api/v1/sessions/{id}/write  向会话写入输入
GET    /api/v1/sessions/{id}/output 读取输出（?since=N&limit=M）
POST   /api/v1/sessions/{id}/resize 调整终端尺寸（列 x 行）
POST   /api/v1/sessions/{id}/signal 发送信号（SIGINT|SIGTERM|SIGKILL|SIGHUP）
```

### WebSocket

```
ws://host/ws/sessions/:id             实时流式输出 + 发送输入
ws://host/ws/sessions/:id?since_seq=N 断线重连 + 输出回放
```

### MCP 工具

| 工具 | 说明 |
|------|------|
| `terminal_exec` | 执行命令，返回退出码 + stdout + 耗时 |
| `terminal_spawn` | 创建交互式会话，返回会话 ID |
| `terminal_write` | 向会话写入输入 |
| `terminal_read` | 读取缓冲输出（支持分页） |
| `terminal_signal` | 发送 POSIX 信号 |
| `terminal_resize` | 调整终端尺寸 |
| `terminal_kill` | 终止并清理会话 |
| `terminal_list` | 列出所有活跃会话 |
| `terminal_info` | 获取会话详情 |

## 配置

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `HOST` | 127.0.0.1 | 监听地址 |
| `PORT` | 8080 | 监听端口 |
| `MAX_SESSIONS` | 50 | 最大并发会话数 |
| `SESSION_TTL` | 5m | 空闲会话超时 |
| `MAX_EXEC_TIMEOUT` | 120s | 默认 exec 命令超时 |
| `MAX_OUTPUT_LINES` | 1000 | 环形缓冲区行数 |

## Docker

```bash
docker compose up -d
curl http://localhost:8080/health
```

## 开发

```bash
make build      # 编译
make run        # 编译 + 运行
make test       # 运行 Go 测试
make vet        # 静态分析
make fmt        # 格式化

# 竞态检测
go test -race ./...

# 集成测试（需先启动服务）
./terminal-endpoint &
python3 test_integration.py
python3 test_agent_workflow.py
```

## 文档

- [快速入门](docs/quickstart_zh.md) — 5 分钟上手
- [API 参考](docs/api_zh.md) — 完整端点和 MCP 工具参考
- [SKILL.md](SKILL.md) — Agent 工具使用指南（含工作流模式）
- [架构方案](.sisyphus/plans/architecture.md) — 设计决策和数据流

## License

MIT
