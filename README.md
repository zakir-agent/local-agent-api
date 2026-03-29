# claude-local-api

将本地 Claude Code CLI 封装为 OpenAI 和 Anthropic 兼容的 HTTP API，方便通过 Chatbox、Cherry Studio 等客户端调用 Claude Max 订阅额度。

## 前置条件

- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) 已安装并登录
- Go 1.21+（仅编译需要）

## 安装

```bash
go install github.com/zakir-agent/claude-local-api@latest
```

或从源码编译：

```bash
git clone https://github.com/zakir-agent/claude-local-api.git
cd claude-local-api
go build
```

## 使用

```bash
# 默认监听 :8080，使用 sonnet 模型
./claude-local-api

# 自定义端口和模型
./claude-local-api -port 9090 -model opus
```

## API

### POST /v1/chat/completions

OpenAI 兼容的聊天补全接口。

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"hello"}]}'
```

### POST /v1/messages

Anthropic Messages API 兼容接口。

```bash
curl http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "x-api-key: any" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model":"claude-sonnet","max_tokens":1024,"messages":[{"role":"user","content":"hello"}]}'
```

### GET /v1/models

返回可用模型列表。

## 客户端配置

在 Chatbox / Cherry Studio 等客户端中：

- **API 地址**: `http://localhost:8080`
- **API Key**: 任意值（不校验）
- **模型**: `claude-sonnet`

## 说明

- 不支持 streaming
- 请求串行处理，同一时间只有一个 CLI 调用
- 仅供本地个人调试使用
- 请求日志记录在 `logs/` 目录下（按天轮转的 JSONL 文件）

## License

[MIT](LICENSE)
