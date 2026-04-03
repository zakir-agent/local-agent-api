# local-agent-api

将本地 **Claude Code** 或 **Cursor Agent** CLI 封装为 OpenAI（Chat Completions、**Responses API**）与 Anthropic 兼容的 HTTP API，便于其他客户端使用本地已登录的订阅额度。直接运行二进制时：默认监听 `:8080`、`-model` 为 **`composer-2-fast`**、`-agent-cli` 默认为 **`cursor`**；使用 Claude 时加 **`-agent-cli claude`**。

## 前置条件

- [Cursor Agent CLI](https://cursor.com/docs/cli/headless)（`agent`；默认 `-agent-cli cursor`）
- 或 [Claude Code CLI](https://docs.anthropic.com/en/docs/claude-code) 已安装并登录（使用 `-agent-cli claude` 时需要）
- Go 1.26+（与 `go.mod` 一致，仅编译需要）

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
# 默认 :8080、-model composer-2-fast、-agent-cli cursor
./local-agent-api

# 自定义端口、模型、Agent
./local-agent-api -port 9090 -model opus -agent-cli claude
```

### 后台运行（关闭终端后仍保留进程）

仓库提供 `nohup` 封装脚本，默认 **`AGENT_CLI=cursor`**、**`MODEL=composer-2-fast`**（可用环境变量覆盖，见 `./scripts/local-deploy.sh --help`）：

```bash
./scripts/local-deploy.sh start    # 先 go build，再 nohup 启动
./scripts/local-deploy.sh status
./scripts/local-deploy.sh stop
./scripts/local-deploy.sh restart
```

日志：`logs/daemon.log`（脚本 stdout/stderr），应用请求日志仍在 `logs/` 下 JSONL。

## API

### POST /v1/chat/completions

OpenAI 兼容的聊天补全接口。

```bash
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4.2","messages":[{"role":"user","content":"hello"}]}'
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

在 Chatbox 等客户端中：

- **API 地址**: `http://localhost:8080`
- **API Key**: 任意值（不校验）
- **模型**: 客户端里可填任意已声明名称（如 `claude-sonnet`）；真正传给 CLI 的模型由启动参数 `-model` 决定（默认 `composer-2-fast`）

若客户端仅支持 OpenAI **Responses** 端点，将 API 根地址指向本服务即可使用 `/v1/responses`。

## 说明

- 不支持 streaming
- 请求串行处理，同一时间只有一个 CLI 调用
- 仅供本地个人调试使用
- 请求日志记录在 `logs/` 目录下（按天轮转的 JSONL 文件）

## License

[MIT](LICENSE)
