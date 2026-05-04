# TerminalEndPoint — 架构方案

## 1. 项目概述

为 AI Agent 提供完善的终端控制端点，支持流式操控。Agent 可以通过此服务创建、管理、操控终端会话，实时获取命令输出，并支持交互式 REPL。

### 核心目标

- **流式 I/O**：WebSocket 实时推送 stdout/stderr，接收 stdin 输入
- **双模式接入**：同时支持 HTTP REST + WebSocket 和 MCP (Model Context Protocol)
- **多会话管理**：并发管理多个独立终端会话，隔离环境变量和工作目录
- **完整 PTY 支持**：终端尺寸调整、信号发送（Ctrl-C、Ctrl-D）、环境变量注入
- **Agent 友好**：超时控制、输出截断、结构化事件、错误分类

## 2. 架构总览

```
┌─────────────────────────────────────────────────────────┐
│                      Agent (Client)                      │
│  ┌──────────┐  ┌──────────┐  ┌───────────────────────┐ │
│  │ MCP Tool │  │ REST API │  │ WebSocket Stream      │ │
│  │ (stdio)  │  │ (HTTP)   │  │ (ws://host/ws/session)│ │
│  └────┬─────┘  └────┬─────┘  └───────────┬───────────┘ │
└───────┼─────────────┼────────────────────┼──────────────┘
        │             │                    │
        ▼             ▼                    ▼
┌─────────────────────────────────────────────────────────┐
│                  TerminalEndPoint Server                 │
│                                                         │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │ MCP Handler │  │  HTTP Router │  │  WS Hub       │  │
│  │ (stdio/json)│  │  (chi/gin)  │  │  (gorilla)   │  │
│  └──────┬──────┘  └──────┬───────┘  └───────┬───────┘  │
│         │                │                   │          │
│         └────────────────┼───────────────────┘          │
│                          ▼                              │
│              ┌───────────────────────┐                  │
│              │   Session Manager     │                  │
│              │  ┌─────────────────┐  │                  │
│              │  │ Session Pool    │  │                  │
│              │  │ (map[id]*Session)│  │                  │
│              │  └────────┬────────┘  │                  │
│              │           │           │                  │
│              │  ┌────────▼────────┐  │                  │
│              │  │  PTY Manager    │  │                  │
│              │  │  (creack/pty)   │  │                  │
│              │  └────────┬────────┘  │                  │
│              │           │           │                  │
│              │  ┌────────▼────────┐  │                  │
│              │  │  Output Buffer  │  │                  │
│              │  │  (ring buffer)  │  │                  │
│              │  └─────────────────┘  │                  │
│              └───────────────────────┘                  │
│                                                         │
│  ┌──────────────────────────────────────────────────┐   │
│  │              Event Bus (channel-based)            │   │
│  │   events: output | exit | resize | error | state │   │
│  └──────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

## 3. 核心组件

### 3.1 Session Manager

管理所有终端会话的生命周期：

```
Session {
    ID          string        // UUID
    Status      SessionState  // creating | running | exited | killed
    PTY         *os.File      // PTY master file descriptor
    Cmd         *exec.Cmd     // 子进程
    WinSize     Winsize       // 终端窗口尺寸
    Env         []string      // 环境变量
    WorkDir     string        // 工作目录
    CreatedAt   time.Time
    ExitCode    int           // -1 表示还在运行
    Buffer      *RingBuffer   // 输出环形缓冲区（保留最近 N 行/字节）
    Subscribers map[string]chan Event  // WebSocket 订阅者
}
```

### 3.2 Event Bus

所有终端事件通过 channel 分发：

```go
type Event struct {
    Type      EventType  // "output" | "exit" | "error" | "state"
    SessionID string
    Data      json.RawMessage
    Timestamp time.Time
}

type EventType string
const (
    EventOutput EventType = "output"       // stdout/stderr 数据
    EventExit   EventType = "exit"         // 进程退出
    EventError  EventType = "error"        // 执行错误
    EventState  EventType = "state_change" // 会话状态变更
)
```

### 3.3 Output Buffer

环形缓冲区，保留最近 N 条输出行（默认 1000 行或 64KB）：

- 新建连接可以回溯历史输出
- 避免内存无限增长
- 支持按时间范围/行号查询

### 3.4 MCP Handler

实现 MCP 协议的 JSON-RPC over stdio：

**Tools 暴露：**
- `terminal_create` — 创建新终端会话
- `terminal_exec` — 在会话中执行命令（非交互）
- `terminal_spawn` — 启动交互式 shell/REPL
- `terminal_write` — 向会话发送输入
- `terminal_read` — 读取会话输出（可指定 offset）
- `terminal_resize` — 调整终端尺寸
- `terminal_signal` — 发送信号（SIGINT, SIGTERM）
- `terminal_kill` — 终止会话
- `terminal_list` — 列出所有会话

## 4. API 设计

### 4.1 REST API

| Method | Path | 描述 |
|--------|------|------|
| `POST` | `/api/v1/sessions` | 创建终端会话 |
| `GET` | `/api/v1/sessions` | 列出所有会话 |
| `GET` | `/api/v1/sessions/:id` | 获取会话详情 |
| `POST` | `/api/v1/sessions/:id/exec` | 执行命令（同步，等待完成）|
| `POST` | `/api/v1/sessions/:id/spawn` | 启动交互式进程 |
| `POST` | `/api/v1/sessions/:id/write` | 向终端写入数据 |
| `GET` | `/api/v1/sessions/:id/output` | 读取输出（支持 offset/limit）|
| `POST` | `/api/v1/sessions/:id/resize` | 调整终端尺寸 |
| `POST` | `/api/v1/sessions/:id/signal` | 发送信号 |
| `DELETE` | `/api/v1/sessions/:id` | 终止并清理会话 |

### 4.2 WebSocket

```
ws://host/ws/sessions/:id?mode=stream
```

**客户端 → 服务端：**
```json
{"type": "write", "data": "ls -la\n"}
{"type": "resize", "cols": 120, "rows": 40}
{"type": "signal", "signal": "SIGINT"}
{"type": "ping"}
```

**服务端 → 客户端：**
```json
{"type": "output", "data": "base64encoded", "stream": "stdout", "seq": 42}
{"type": "exit", "code": 0}
{"type": "error", "message": "session not found"}
{"type": "history", "data": ["line1", "line2"], "next_seq": 3}
{"type": "pong"}
```

> `output.data` 使用 base64 编码，确保二进制数据安全传输。

**连接建立时：**
1. 如果会话已存在且有历史输出，先发送 `history` 事件回放
2. 然后进入实时 streaming 模式

### 4.3 MCP Protocol (JSON-RPC over stdio)

MCP 客户端启动 server 进程后通过 stdin/stdout 通信：

```json
// → 列出工具
{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}

// ← 返回
{"jsonrpc":"2.0","id":1,"result":{"tools":[{
  "name":"terminal_exec",
  "description":"Execute a command in a terminal session and return the output",
  "inputSchema":{
    "type":"object",
    "properties":{
      "session_id":{"type":"string","description":"Session ID"},
      "command":{"type":"string","description":"Command to execute"},
      "timeout":{"type":"integer","description":"Timeout in seconds"},
      "work_dir":{"type":"string","description":"Working directory for this command"}
    },
    "required":["command"]
  }
}]}}

// → 调用工具
{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{
  "name":"terminal_exec",
  "arguments":{"command":"ls -la","session_id":"abc123"}
}}
```

## 5. 会话生命周期

```
                 ┌──────────┐
                 │  CREATED  │  ← POST /sessions
                 └────┬─────┘
                      │
           ┌──────────┼──────────┐
           ▼          ▼          ▼
     POST /exec  POST/spawn  WS connect
           │          │          │
           ▼          ▼          ▼
    ┌─────────┐ ┌──────────┐ ┌──────────┐
    │  执行中  │ │  运行中   │ │  流式中   │
    │ (同步)  │ │ (交互式)  │ │ (实时)   │
    └────┬────┘ └────┬─────┘ └────┬─────┘
         │           │            │
         ▼           ▼            ▼
    ┌─────────────────────────────────┐
    │            EXITED               │
    │  (exit_code 记录)               │
    └────────────┬────────────────────┘
                 │
                 ▼
    ┌─────────────────────────────────┐
    │           CLEANED               │
    │  (DELETE 或 TTL 过期)            │
    └─────────────────────────────────┘
```

**TTL 策略：**
- 无活动会话默认 5 分钟后自动清理
- 可配置 `idle_timeout`
- 可配置 `max_lifetime`（最长存活时间）

## 6. 数据模型

```go
// 创建会话请求
type CreateSessionRequest struct {
    WorkDir string            `json:"work_dir,omitempty"`
    Env     map[string]string `json:"env,omitempty"`
    Shell   string            `json:"shell,omitempty"`   // 默认 /bin/bash
    Cols    int               `json:"cols,omitempty"`     // 默认 80
    Rows    int               `json:"rows,omitempty"`     // 默认 24
    Label   string            `json:"label,omitempty"`    // 可读标签
    TTL     int               `json:"ttl_seconds,omitempty"` // 默认 300
}

// 执行命令请求
type ExecRequest struct {
    Command  string            `json:"command"`
    Timeout  int               `json:"timeout_seconds,omitempty"` // 默认 30
    WorkDir  string            `json:"work_dir,omitempty"`
    Env      map[string]string `json:"env,omitempty"`
}

// 执行命令响应
type ExecResponse struct {
    ExitCode int    `json:"exit_code"`
    Stdout   string `json:"stdout"`
    Stderr   string `json:"stderr"`
    Duration int64  `json:"duration_ms"`
    Truncated bool  `json:"truncated,omitempty"` // 输出被截断时为 true
}

// 会话详情
type SessionInfo struct {
    ID        string       `json:"id"`
    Status    string       `json:"status"`
    WorkDir   string       `json:"work_dir"`
    Shell     string       `json:"shell"`
    ExitCode  int          `json:"exit_code,omitempty"`
    CreatedAt time.Time    `json:"created_at"`
    UpdatedAt time.Time    `json:"updated_at"`
    Label     string       `json:"label,omitempty"`
    WinSize   WinSize      `json:"win_size"`
}

type WinSize struct {
    Cols int `json:"cols"`
    Rows int `json:"rows"`
}
```

## 7. 目录结构

```
TerminalEndPoint/
├── cmd/
│   └── terminal-endpoint/
│       ├── main.go              # HTTP/MCP 双模式入口
│       └── mcp.go               # MCP stdio 模式入口
├── internal/
│   ├── server/
│   │   ├── server.go            # HTTP Server 初始化
│   │   ├── router.go            # 路由注册
│   │   └── middleware.go        # 中间件（CORS, logging, auth）
│   ├── handler/
│   │   ├── session.go           # REST session handler
│   │   ├── exec.go              # REST exec handler
│   │   ├── websocket.go         # WebSocket handler
│   │   └── health.go            # Health check
│   ├── mcp/
│   │   ├── server.go            # MCP JSON-RPC server
│   │   ├── tools.go             # MCP tool definitions
│   │   └── handler.go           # MCP tool handlers
│   ├── session/
│   │   ├── manager.go           # Session Manager
│   │   ├── session.go           # Session 结构和方法
│   │   ├── buffer.go            # Ring Buffer
│   │   └── event.go             # Event 定义
│   ├── pty/
│   │   ├── pty.go               # PTY 创建和管理
│   │   └── resize.go            # 终端尺寸调整
│   └── config/
│       └── config.go            # 配置管理
├── pkg/
│   └── protocol/
│       ├── rest.go              # REST API 类型
│       ├── websocket.go         # WebSocket 协议类型
│       └── mcp.go               # MCP 协议类型
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## 8. 技术选型

| 组件 | 选型 | 理由 |
|------|------|------|
| HTTP Router | `chi` | 轻量、惯用、兼容 net/http |
| WebSocket | `gorilla/websocket` | 最成熟的 Go WebSocket 库 |
| PTY | `creack/pty` | Go 标准 PTY 库 |
| UUID | `google/uuid` | 会话 ID 生成 |
| Config | `spf13/viper` | 配置管理 |
| Logging | `slog` (stdlib) | Go 1.21+ 标准日志库 |
| CLI | `cobra` | 命令行参数解析 |

## 9. 流式输出协议详解

### 输出分块策略

- PTY 读取循环每次读 4096 字节
- 多个 chunk 可在单次 flush 中合并发送（最多 16KB 或 50ms 间隔）
- 每次发送附带单调递增的 `seq` 序号
- 客户端可据此检测丢包（通过 WebSocket 重连 + `since_seq` 参数恢复）

### WebSocket 重连恢复

```
GET /ws/sessions/:id?since_seq=42
```

服务端从环形缓冲区中查找 seq >= 42 的所有事件并回放，然后继续实时推送。

### 输出截断

当非流式调用（REST exec）时，输出超过限制（默认 100KB）后截断并返回 `truncated: true`。流式模式不受此限。

## 10. 实现阶段

### Phase 1: 核心 PTY + Session 管理 (MVP)
- [ ] PTY 创建/销毁
- [ ] Session Manager（创建、列表、详情、删除）
- [ ] 基本的命令执行（同步 exec）
- [ ] 输出缓冲区

### Phase 2: WebSocket 流式操控
- [ ] WebSocket handler
- [ ] 实时 stdin/stdout 双向流
- [ ] 输出分块和 seq 机制
- [ ] 重连恢复（since_seq）
- [ ] 交互式 REPL 支持

### Phase 3: REST API 完善
- [ ] 完整 CRUD API
- [ ] 信号发送（SIGINT, SIGTERM, SIGKILL）
- [ ] 终端 resize
- [ ] 超时和 TTL 自动清理
- [ ] 健康检查和监控端点

### Phase 4: MCP 集成
- [ ] MCP JSON-RPC server（stdio 模式）
- [ ] 工具定义和注册
- [ ] MCP streaming 资源（可选）

### Phase 5: 生产就绪
- [ ] 配置管理（YAML/环境变量）
- [ ] 并发安全审计
- [ ] 优雅关闭
- [ ] 日志和错误处理完善
- [ ] Docker 部署支持
- [ ] 基础测试套件

## 11. 关键设计决策

1. **Go 选型**：高性能并发，内存安全，单二进制部署，天然适合 IO 密集型
2. **creack/pty**：Go 生态标准 PTY 实现，稳定可靠
3. **一个 Session = 一个 PTY + 一个 Shell 进程**：生命周期绑定，简单直观
4. **Ring Buffer 而非全量存储**：控制内存，Server 无状态重启友好
5. **双模式接入**：MCP 适合 Claude/本地 agent，HTTP/WS 适合远程/Web agent
6. **seq + since_seq 恢复**：简单可靠的断线重连机制
