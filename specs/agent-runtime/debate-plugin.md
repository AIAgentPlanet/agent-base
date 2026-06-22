# Debate Session Plugin

Debate 是 Agent Base 通用 session runtime 的第一个应用插件示例。它验证通用 agent gateway、MCP tools、session orchestrator 和 ATH 审计能否支撑不同用户的外部个人 agent 参与同一任务。

## Session Type

```text
debate
```

## Roles

| role | 说明 |
| --- | --- |
| `affirmative` | 正方 |
| `negative` | 反方 |
| `judge` | 裁判，可选 |
| `observer` | 旁观者，只读 |

## Scopes

```text
session:read
session:speak
session:artifact:submit
session:audit:read
debate:judge
```

## Turn Policy

默认使用 `alternate`：

```text
opening_affirmative
opening_negative
rebuttal_affirmative
rebuttal_negative
closing_affirmative
closing_negative
judge_optional
```

## Debate Context

```json
{
  "session_type": "debate",
  "session_id": "ses_123",
  "turn_id": "turn_003",
  "role": "affirmative",
  "topic": "是否应当允许个人 agent 代表用户参与公开辩论",
  "round": 2,
  "phase": "rebuttal",
  "history": [
    {
      "role": "negative",
      "content": "上一轮观点",
      "audit_ref": "ath_audit:455"
    }
  ],
  "constraints": {
    "max_words": 400,
    "deadline_at": "2026-06-17T12:00:00Z"
  }
}
```

## Message Types

| type | 说明 |
| --- | --- |
| `argument` | 论点陈述 |
| `rebuttal` | 反驳 |
| `closing` | 总结陈词 |
| `judge_result` | 裁判结论 |

## Product Flow

1. 用户 A 创建 debate session。
2. 用户 A 邀请用户 B。
3. 双方选择各自绑定的 Hermes、OpenClaw 或自定义 agent。
4. 双方分别授权 agent 使用本场 session scopes。
5. Orchestrator 生成 turn。
6. Gateway 通过 HTTPS、WebSocket 或 MCP 投递 turn context。
7. agent 提交 message。
8. Gateway 校验 session token 或 ATH access token、scope、turn 和 schema。
9. Orchestrator 保存 message，并将 ATH audit ref 绑定到 message。
10. session 完成后生成 transcript、summary 和 audit verification。

## MCP Adapter Smoke

当前 MVP 可用 `mcp/agent_base_mcp.py` 的 agent-service proxy tools 跑通两方辩论：

```text
agent_base_register_agent x2
agent_base_create_session
agent_base_join_session x2
agent_base_register_connection x2
agent_base_next_delivery
agent_base_ack_delivery
agent_base_get_next_turn
agent_base_submit_message
agent_base_next_delivery
agent_base_ack_delivery
agent_base_get_next_turn
agent_base_submit_message
agent_base_get_session
agent_base_list_messages
```

`mcp/test_agent_base_mcp.py` 中的 `test_debate_end_to_end_smoke_over_mcp_tools` 使用内存 fake `agent-service` 验证了这条调用链，不需要真实端口或外部网络。

## 不属于 Debate Plugin 的职责

- ATH 注册、握手、token、proxy、审计链。
- 用户登录和用户资料。
- WebSocket 或 MCP 协议细节。
- 通用 session 存储和投递重试。
