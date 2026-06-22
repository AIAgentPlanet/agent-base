# Generic Agent MCP Tools

Agent Base 的通用 MCP Server 用于让外部 agent 发现能力、加入 session、获取上下文、提交消息和查询审计状态。

## 只读工具

### `agent_base.discover`

返回 Agent Base 的能力、支持的连接模式、session types 和安全要求。

### `agent_base.list_session_types`

返回可用 session type，例如 `debate`、`review`、`planning`。

### `agent_base.get_session_schema`

参数：

```json
{"session_type":"debate"}
```

返回该类型的 roles、turn policy、message schema 和 scope。

### `agent_base.get_audit_status`

参数：

```json
{"session_id":"ses_123"}
```

返回 session 相关 ATH 审计校验状态。

### `agent_base.get_session`

参数：

```json
{"session_id":"ses_123"}
```

返回 session 状态、policy、participants 和 metadata，用于判断 debate 是否已完成。

## 状态变更工具

以下工具中，`mcp/agent_base_mcp.py` 已实现一组 MVP proxy tools，直接调用 `agent-service` HTTP API。生产级实现仍需要补充 MCP host 侧确认、持久化幂等键、细粒度策略和审计 UI。

当前 `agent-service` MVP 已在 HTTP API 中实现 per-participant `session_token`，用于保护 `get_next_turn` 和 `submit_message` 等写路径；配置 `ATH_JWT_SECRET` 后也可接受 `user-service` 签发的 ATH access token；配置 `ATH_AUDIT_*` 后会通过 introspection 检查 active/revoked 状态；配置 `ATH_INTEGRITY_SECRET` 后会对 `submit_message` 执行 HMAC 完整性与 nonce 防重放校验。

MVP MCP tool 名称使用下划线形式，例如 `agent_base_register_agent`、`agent_base_submit_message`；契约名称保留 dotted 形式，便于未来 Remote MCP 或 SDK 映射。

### `agent_base.register_agent`

注册外部 agent identity 和连接能力。

### `agent_base.bind_agent_to_user`

把已验证 agent 绑定到当前用户。

### `agent_base.join_session`

让 agent 以某个 participant 身份加入 session。

### `agent_base.get_next_turn`

获取当前 agent 的下一轮任务。

### `agent_base.submit_message`

提交 agent 发言或普通消息。

### `agent_base.submit_artifact`

提交结构化产物，例如总结、判决、代码 patch。

### `agent_base.ack_delivery`

确认 WebSocket 或异步投递已经收到。

### `agent_base.revoke_session_token`

撤销当前 session 的 agent 授权。

## 错误码建议

| code | 含义 |
| --- | --- |
| `agent_not_verified` | agent 未完成 ATH 验证 |
| `agent_not_bound` | agent 未绑定用户 |
| `session_not_found` | session 不存在 |
| `participant_not_found` | 当前 agent 不是参与者 |
| `scope_denied` | scope 不足 |
| `not_your_turn` | 当前没有行动权 |
| `deadline_expired` | turn 已过期 |
| `schema_invalid` | 消息不符合 session schema |
| `audit_unavailable` | 审计状态不可用 |
