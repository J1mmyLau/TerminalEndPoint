# API 参考

TerminalEndPoint 提供两种接口：REST + WebSocket HTTP API，以及 MCP（Model Context Protocol）JSON-RPC over stdio 接口。

## HTTP REST API

Base URL: `http://host:port`

所有请求和响应均为 JSON。时间戳格式为 RFC 3339。

---

### 会话管理

#### 创建会话

```
POST /api/v1/sessions
```

**请求体：**
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

| 字段 | 类型 | 默认值 | 说明 |
|-------|------|--------|------|
| label | string | "" | 可读标签 |
| work_dir | string | "" | 工作目录 |
| shell | string | "/bin/bash" | Shell 路径 |
| cols | uint16 | 80 | 终端列数 |
| rows | uint16 | 24 | 终端行数 |
| env | map | {} | 额外环境变量（与服务器环境合并） |

**响应：** `201 Created`
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

状态值：`running`（运行中）、`exited`（已退出）、`killed`（已终止）、`error`（错误）。运行中时 exit_code 为 -1。

---

#### 列出会话

```
GET /api/v1/sessions
```

**响应：** `200 OK`
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

#### 获取会话详情

```
GET /api/v1/sessions/{id}
```

**响应：** `200 OK` — 与创建会话返回格式相同。

**错误：** `404` — 会话不存在。

---

#### 终止会话

```
DELETE /api/v1/sessions/{id}
```

**响应：** `200 OK`
```json
{"status": "killed", "id": "550e8400-..."}
```

先发送 SIGTERM，关闭 PTY，等待退出，必要时强制 kill。

---

### 命令执行

#### 执行命令

```
POST /api/v1/sessions/{id}/exec
```

`{id}` 路径段会被忽略 —— 每次 exec 创建一个临时会话，执行 `/bin/sh -c <command>` 后退出，会话自动清理。

**请求体：**
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

| 字段 | 类型 | 默认值 | 说明 |
|-------|------|--------|------|
| command | string | （必填） | 要执行的 shell 命令 |
| timeout_seconds | int | 120 | 最大执行时间（秒） |
| work_dir | string | "" | 工作目录 |
| env | map | {} | 额外环境变量 |

**响应：** `200 OK`
```json
{
  "exit_code": 0,
  "stdout": "编译输出...\n程序输出...\n",
  "stderr": "",
  "duration_ms": 1523,
  "truncated": false
}
```

`truncated: true` 表示输出超过 100KB 缓冲区限制。返回的 `stdout` 包含截断前捕获的内容。超时被终止时 `exit_code` 为 -1。

**错误：** `400` — 缺少 command 参数或 JSON 无效。`500` — 创建会话失败。

---

### 会话 I/O

#### 写入输入

```
POST /api/v1/sessions/{id}/write
```

**请求体：**
```json
{
  "data": "ls -la\n"
}
```

末尾加 `\n` 才会在 shell 中执行命令。

**响应：** `200 OK`
```json
{"status": "written"}
```

**错误：** `404` — 会话不存在或已停止。

---

#### 读取输出

```
GET /api/v1/sessions/{id}/output?since=0&limit=50
```

查询参数：

| 参数 | 类型 | 默认值 | 说明 |
|-------|------|--------|------|
| since | uint64 | 0 | 返回 seq >= 此值的条目 |
| limit | int | 0（全部） | 最大返回条目数 |

**响应：** `200 OK`
```json
{
  "entries": [
    {"seq": 1, "data": "SGVsbG8gd29ybGQ="},
    {"seq": 2, "data": "YW5vdGhlciBsaW5l"}
  ],
  "latest_seq": 2
}
```

`data` 为 base64 编码的输出。解码后得到原始字节。

---

#### 调整终端尺寸

```
POST /api/v1/sessions/{id}/resize
```

**请求体：**
```json
{
  "cols": 120,
  "rows": 40
}
```

**响应：** `200 OK`
```json
{"status": "resized"}
```

---

#### 发送信号

```
POST /api/v1/sessions/{id}/signal
```

**请求体：**
```json
{
  "signal": "SIGINT"
}
```

有效信号：`SIGINT`、`SIGTERM`、`SIGKILL`、`SIGHUP`、`SIGQUIT`。

**响应：** `200 OK`
```json
{"status": "signaled", "signal": "SIGINT"}
```

---

### 健康检查

```
GET /health
```

**响应：** `200 OK`
```json
{"status": "ok", "sessions": 3}
```

---

## WebSocket API

### 连接

```
ws://host/ws/sessions/{id}
ws://host/ws/sessions/{id}?since_seq=42
```

带 `since_seq` 连接时，服务器先回放缓冲区中 seq >= 该值的输出条目，然后进入实时流模式。

### 客户端 → 服务器消息

```json
{"type": "write", "data": {"data": "ls -la\n"}}
{"type": "resize", "data": {"cols": 120, "rows": 40}}
{"type": "signal", "data": {"signal": "SIGINT"}}
```

### 服务器 → 客户端消息

**历史回放（带 since_seq 连接时）：**
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

**实时输出：**
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

**进程退出：**
```json
{
  "type": "exit",
  "data": {"code": 0}
}
```

**错误：**
```json
{
  "type": "error",
  "data": {"message": "会话不存在"}
}
```

所有输出消息中的 `data` 字段均为 base64 编码。

### 心跳

服务器每 30 秒发送 WebSocket ping 帧。客户端应回复 pong 帧。

---

## MCP 协议（JSON-RPC over stdio）

启动 MCP 模式：`./terminal-endpoint mcp`

服务器从 stdin 读取 JSON-RPC 请求（每行一个，换行符分隔），将响应写入 stdout。

### 生命周期

**初始化：**
```json
// → 请求
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}

// ← 响应
{
  "jsonrpc":"2.0","id":1,
  "result":{
    "protocolVersion":"2024-11-05",
    "capabilities":{"tools":{"listChanged":false}},
    "serverInfo":{"name":"terminal-endpoint","version":"0.1.0"}
  }
}
```

### 工具发现

**列出工具：**
```json
// → 请求
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}

// ← 响应
{"jsonrpc":"2.0","id":2,"result":{"tools":[...]}}
```

### 工具调用

**调用工具：**
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

**响应：**
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

### 工具参考

详见 [SKILL.md](../SKILL.md) 中的逐工具文档（含示例）。

### 错误处理

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

错误码：
| 码 | 含义 |
|------|------|
| -32601 | 方法未找到 |
| -32000 | 工具执行错误（消息包含详细信息） |

---

## 错误响应格式

所有 REST 错误遵循以下格式：
```json
{
  "error": "人类可读的消息",
  "code": 400
}
```

HTTP 状态码：
| 码 | 含义 |
|------|------|
| 200 | 成功 |
| 201 | 会话已创建 |
| 400 | 请求错误（无效 JSON、缺少字段） |
| 404 | 会话未找到 |
| 500 | 内部错误（PTY 失败、超过最大会话数） |

---

## 会话生命周期

```
已创建 ──exec──▶ 执行命令 → 退出 → 自动清理
已创建 ──spawn─▶ 运行中 ──write/read──▶ （交互式操作）
运行中 ──signal─▶ 已退出（或 已终止）
已退出 ──TTL(5分钟)─▶ 自动清理
运行中 ──DELETE─▶ 已终止 → 自动清理
```

会话在 5 分钟无活动后自动清理（可通过 `SESSION_TTL` 配置）。
