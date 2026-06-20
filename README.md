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

REST:

```bash
curl -X POST http://localhost:8080/api/v1/agent/chat \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"demo","session_id":"default","message":"帮我制定 Rust 学习计划"}'
```

WebSocket:

```text
ws://localhost:8080/ws/v1/agent
```
