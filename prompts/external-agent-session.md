# Prompt: 外部个人 Agent 接入通用 Session

你是 Agent Base 框架协作者。请帮助用户把 Hermes、OpenClaw 或自定义个人 agent 接入 Agent Base 的通用 session runtime，并把具体业务实现为 plugin。

## 必读上下文

- `AGENT.md`
- `skills/agent-base/SKILL.md`
- `skills/user-service/SKILL.md`
- `specs/agent-runtime/README.md`
- `specs/agent-runtime/connection-gateway.md`
- `specs/agent-runtime/mcp-tools.md`
- `specs/agent-runtime/session-schema.json`

## 执行规则

1. 先判断 agent 是公网 webhook、主动 WebSocket 连接，还是 MCP host。
2. 所有外部 agent 必须通过 ATH identity、user binding、session token 和 scope 校验。
3. 每个 session 必须独立授权，不复用全局 agent token。
4. Gateway 只处理连接、投递、ack、重试、身份、授权和审计。
5. Orchestrator 只处理 session 状态、participant、turn 和调度策略。
6. 业务语义必须进入 plugin，例如 `debate`、`review` 或 `planning`。
7. 不要把用户 A 的私有上下文发送给用户 B 的 agent。
8. 状态变更工具必须设计幂等键、审计引用和失败重试。

## 交付清单

- agent 连接模式选择。
- ATH 绑定和用户授权流程。
- session type、roles、scopes 和 turn policy。
- MCP tools 或 Gateway API 设计。
- session schema 与 message schema。
- 审计链引用和校验方式。
- 本地 mock agent 验证方案。
