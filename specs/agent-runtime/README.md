# Agent Runtime Contracts

本目录定义 Agent Base 的通用外部 agent 接入契约。目标是让 OpenClaw、Hermes、Codex、Claude Desktop 或自定义 agent 以统一方式接入不同业务应用，而不是为单个辩论产品写死协议。

## 分层

```text
External Agent
  | HTTPS / WebSocket / MCP
  v
Agent Connection Gateway
  | ATH identity, user binding, session token, audit
  v
Agent MCP Server
  | generic session tools
  v
Session Orchestrator
  | session policy and turn scheduling
  v
Application Plugin
```

## 通用层职责

通用层只关心：

- 哪个 agent 连接了 Agent Base。
- 这个 agent 是否通过 ATH 验证。
- 这个 agent 被哪个用户绑定或授权。
- 这个 agent 可以访问哪个 session。
- 当前是否轮到这个 agent 行动。
- 消息是否符合 session schema。
- 调用是否有审计证据。

通用层不关心：

- 辩论胜负。
- 代码评审标准。
- 研究任务如何拆解。
- 具体业务 prompt。

这些应由 application plugin 定义。

## 核心概念

| 概念 | 说明 |
| --- | --- |
| Agent | 外部个人 agent runtime，例如 Hermes、OpenClaw 或自定义 agent |
| Agent Binding | 用户和 agent identity 的绑定关系 |
| Connection | agent 与 Agent Base 的在线连接，可为 webhook、WebSocket 或 MCP |
| Session | 一次多 agent 协作或对抗任务 |
| Participant | session 中的参与者，绑定 user、agent、role 和 scopes |
| Turn | 调度器分配给某个 participant 的行动机会 |
| Message | agent 或用户提交的内容 |
| Artifact | agent 生成的结构化产物，例如报告、代码、裁判结果 |
| Audit Ref | 指向 ATH 审计链记录的引用 |

## 首批规格

- `connection-gateway.md`：外部 agent 连接层契约。
- `mcp-tools.md`：通用 MCP tools 契约。
- `session-schema.json`：通用 session 数据模型。
- `debate-plugin.md`：辩论作为第一个 application plugin 的示例。

## 安全原则

1. 每个 agent 必须有稳定 identity。
2. 每个用户必须显式绑定或授权自己的 agent。
3. 每个 session 使用独立授权，不复用泛用 token。
4. scope 必须最小化。
5. 所有状态变化必须可审计。
6. Gateway 负责认证和审计，业务服务只接收已验证上下文。
7. MVP 可以使用 per-participant session token 验证写操作；生产版应替换为 ATH/session token。
