# local-agent-api

将本地 **Claude Code** 或 **Cursor Agent** CLI 封装为 OpenAI（Chat Completions、**Responses API**）与 Anthropic 兼容的 HTTP API，便于其他客户端使用本地已登录的订阅额度；默认后端为 Claude（`-agent-cli claude`），亦可切换 Cursor（`-agent-cli cursor`）。

## 前置条件

- [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) 已安装并登录（`-agent-cli claude`，默认）
- 或 [Cursor Agent CLI](https://cursor.com/docs/cli/headless)（`agent`，非交互需 `-p` / `--print`；本仓库用 `-agent-cli cursor` 封装）
- Go 1.21+（仅编译需要）

## 安装

```bash
go install github.com/zakir-agent/local-agent-api@latest
```

或从源码编译：

```bash
git clone https://github.com/zakir-agent/local-agent-api.git
cd local-agent-api
go build -o local-agent-api .
```

## 使用

```bash
# 默认监听 :8080，使用 sonnet 模型
./local-agent-api

# 自定义端口和模型
./local-agent-api -port 9090 -model opus

# 使用 Cursor Agent（headless：stdin 提示词 + JSON 输出）
./local-agent-api -agent-cli cursor -model sonnet-4
```

## API

### POST /v1/chat/completions

OpenAI 兼容的聊天补全接口。

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"messages":[{"role":"user","content":"hello"}]}'
```

### POST /v1/responses

OpenAI [Responses API](https://platform.openai.com/docs/api-reference/responses/create) 兼容接口。

```bash
curl http://localhost:8080/v1/responses \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-5.4","input": "Write a short bedtime story about a unicorn."}'
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
- **模型**: `claude-sonnet`（走 Responses 模式的客户端请使用对应 Base URL，并选任意已声明模型名）

若客户端仅支持 OpenAI **Responses** 端点，将 API 根地址指向本服务即可使用 `/v1/responses`。

## 说明

- 不支持 streaming
- 请求串行处理，同一时间只有一个 CLI 调用
- 仅供本地个人调试使用
- 请求日志记录在 `logs/` 目录下（按天轮转的 JSONL 文件）

## License

[MIT](LICENSE)
