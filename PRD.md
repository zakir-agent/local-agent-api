# claude-local-api PRD

## 概述

将本地 Claude Code CLI 封装为 OpenAI 兼容的 HTTP API，方便通过各类客户端工具（Chatbox、Cherry Studio 等）调用 Claude Max 订阅额度进行本地调试。

## 背景

- 用户拥有 Claude Max 订阅，Claude Code CLI 已登录可用
- 无 Anthropic API key，无法直接使用 Claude API
- 需要一个本地 HTTP 服务，将 CLI 的 `claude -p` 能力暴露为标准 API 接口

## 目标

- 提供 OpenAI 兼容的 `/v1/chat/completions` 接口
- 零配置启动，开箱即用
- 仅供本地个人调试使用

## 非目标

- 不支持 streaming
- 不支持多模型切换（固定使用 sonnet）
- 不提供认证机制
- 不提供 `/v1/models` 等其他 OpenAI 接口
- 不支持多用户/生产环境部署

## 技术方案

### 架构

```
客户端 --HTTP--> claude-local-api (Go, :8080) --exec--> claude -p --model sonnet
```

### API 规格

**POST /v1/chat/completions**

请求体（兼容 OpenAI 格式，仅处理必要字段）：

```json
{
  "messages": [
    {"role": "system", "content": "You are a helpful assistant."},
    {"role": "user", "content": "Hello"},
    {"role": "assistant", "content": "Hi there!"},
    {"role": "user", "content": "How are you?"}
  ]
}
```

- `messages`: 必填，对话历史数组
- `model`: 可选，忽略（固定 sonnet）
- 其他 OpenAI 字段（temperature、max_tokens 等）：忽略

响应体（兼容 OpenAI 格式）：

```json
{
  "id": "chatcmpl-xxxx",
  "object": "chat.completion",
  "created": 1234567890,
  "model": "claude-sonnet",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "I'm doing well, thank you!"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 0,
    "completion_tokens": 0,
    "total_tokens": 0
  }
}
```

注：`usage` 字段填 0，因为 CLI 不返回 token 用量。

### 消息拼接策略

将 messages 数组拼接为纯文本传给 `claude -p`：

```
[system] You are a helpful assistant.

[user] Hello

[assistant] Hi there!

[user] How are you?
```

### CLI 调用方式

```bash
claude -p --model sonnet --output-format json "<拼接后的消息>"
```

- 使用 `--output-format json` 获取结构化输出
- 每次请求独立调用，无 session 复用

### 错误处理

| 场景 | HTTP 状态码 | 说明 |
|------|------------|------|
| messages 为空 | 400 | Bad Request |
| claude CLI 不存在 | 500 | CLI not found |
| claude 执行超时 | 504 | Gateway Timeout |
| claude 执行失败 | 502 | Bad Gateway |

超时时间默认 120 秒。

## 项目结构

```
claude-local-api/
  main.go          # 入口、HTTP server
  handler.go       # 请求处理、响应构建
  claude.go        # CLI 调用封装
  types.go         # OpenAI 兼容的请求/响应类型定义
  go.mod
  PRD.md
  README.md
```

## 技术栈

- Go 标准库（net/http, os/exec, encoding/json）
- 无第三方依赖

## 验证方式

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"hello"}]}'
```
