# Learning Agent

综合型学习 Agent 框架，支持 CLI、RESTful API、WebSocket 三种入口。

## 目标能力

- 意图识别：学习计划、资料问答、练习生成、复盘总结。
- DAG 编排：用状态化 DAG 串联记忆读取、规划、技能执行、记忆写入。
- Skill 系统：通过统一注册中心扩展学习技能。
- 模型调度层：通过 provider/router 接口隔离具体模型供应商。
- 记忆系统：短期会话记忆和长期用户记忆分层。
- 知识库：预留向量检索接口，后续接入 Qdrant。
- 存储：PostgreSQL 作为结构化数据主存储。

## 快速开始

```bash
go mod tidy
go run ./cmd/learning-agent chat "我想三个月学完 Go，每天一小时"
go run ./cmd/learning-agent server
```

也可以使用 Makefile：

```bash
make test
make chat
make dev
```

`make dev` 会启动同一个 Gin 服务，同时提供 REST 和 WebSocket：

```text
REST:      http://localhost:8080/api/v1/agent/chat
WebSocket: ws://localhost:8080/ws/v1/agent
CLI:       make chat CHAT_MESSAGE="帮我制定 Go 学习计划"
```

如果默认 `:8080` 被占用，可以指定临时端口：

```bash
make dev DEV_ADDR=:8081
```

## 本地存储和 PostgreSQL

默认不需要数据库，`.env.example` 中的配置会把会话记忆写到本地文件：

```bash
MEMORY_STORE=local
LOCAL_DATA_PATH=data/memories.jsonl
LOCAL_MESSAGES_PATH=data/messages.jsonl
```

`data/` 已加入 `.gitignore`，不会提交到 Git。

如果要使用 PostgreSQL：

```bash
cp .env.example .env
```

把 `.env` 中的存储切换为任意可访问的 PostgreSQL：

```bash
MEMORY_STORE=postgres
DATABASE_URL=postgres://learning_agent:learning_agent@localhost:55432/learning_agent?sslmode=disable
```

然后执行迁移：

```bash
make migrate
```

如果只是本地开发，需要一个临时 PostgreSQL，可以使用 Docker helper：

```bash
make local-pg-up
make migrate
make local-pg-psql
make local-pg-logs
make local-pg-down
```

## 接入 DeepSeek V4

复制配置模板并填写 API key：

```bash
cp .env.example .env
```

`.env` 不会提交到 Git。配置示例：

```bash
MODEL_PROVIDER=deepseek
DEEPSEEK_API_KEY=sk-xxx
DEEPSEEK_BASE_URL=https://api.deepseek.com
DEEPSEEK_MODEL=deepseek-v4-flash
DEEPSEEK_REASONING_MODEL=deepseek-v4-pro
DEEPSEEK_REASONING_EFFORT=medium
DEEPSEEK_THINKING_ENABLED=false
```

当前 Go 程序不自动读取 `.env` 文件，运行前先导入环境变量：

```bash
set -a
source .env
set +a
```

然后运行真实模型对话：

```bash
go run ./cmd/learning-agent chat "请帮我制定一个 Go 并发学习计划"
```

使用 Makefile 时会自动加载 `.env`：

```bash
make chat
make dev
```

模型选择策略：

- 普通问答和练习默认使用 `deepseek-v4-flash`。
- 学习计划和复盘默认使用 `deepseek-v4-pro`。
- 未设置 `MODEL_PROVIDER=deepseek` 时默认使用 mock provider，方便本地测试。

REST:

```bash
curl -X POST http://localhost:8080/api/v1/agent/chat \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"demo","session_id":"default","message":"帮我制定 Rust 学习计划"}'
```

REST SSE 流式接口：

```bash
curl -N -X POST http://localhost:8080/api/v1/agent/chat/stream \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"demo","session_id":"default","message":"帮我制定 Rust 学习计划"}'
```

WebSocket:

```text
ws://localhost:8080/ws/v1/agent
```
