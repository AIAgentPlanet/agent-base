# Agent Base MCP Server

`mcp/agent_base_mcp.py` 是一个轻量 MCP stdio 服务器，用于让外部 AI Agent 读取 Agent Base 上下文，并通过 `agent-service` proxy tools 参与 session。

## 启动

```bash
python3 mcp/agent_base_mcp.py
```

可选环境变量：

```bash
AGENT_BASE_AGENT_SERVICE_URL=http://localhost:8090
```

CLI 可输出客户端配置：

```bash
python3 cli/agent-base mcp config --format json
```

## 工具

| 工具 | 说明 |
| --- | --- |
| `agent_base_context` | 返回 Agent Base 已实现能力、边界和入口文件 |
| `list_prompts` | 列出 prompt 模板 |
| `read_prompt` | 读取指定 prompt，参数：`name` |
| `list_skills` | 列出 skill |
| `read_skill` | 读取指定 skill，参数：`name` |
| `user_service_endpoints` | 返回用户与 OAuth 接口清单 |
| `ath_endpoints` | 返回 ATH 接口清单 |
| `agent_service_endpoints` | 返回 agent-service MVP 接口清单 |
| `agent_runtime_discover` | 返回通用外部 agent runtime 能力 |
| `list_connection_modes` | 列出 HTTPS webhook、WebSocket、MCP 等连接模式 |
| `list_session_types` | 列出通用 session type |
| `list_runtime_tools` | 列出通用 runtime MCP tool 契约 |
| `read_runtime_spec` | 读取 `specs/agent-runtime/` 中的规格文件 |
| `agent_base_register_agent` | 代理 `POST /api/v1/agents` |
| `agent_base_create_session` | 代理 `POST /api/v1/sessions` |
| `agent_base_join_session` | 代理 `POST /api/v1/sessions/:id/participants` |
| `agent_base_get_session` | 代理 `GET /api/v1/sessions/:id` |
| `agent_base_register_connection` | 代理 `POST /api/v1/connections` |
| `agent_base_next_delivery` | 代理 `GET /api/v1/connections/:id/deliveries/next` |
| `agent_base_ack_delivery` | 代理 `POST /api/v1/connections/:id/deliveries/:deliveryId/ack` |
| `agent_base_get_next_turn` | 代理 `GET /api/v1/sessions/:id/participants/:participantId/next-turn` |
| `agent_base_submit_message` | 代理 `POST /api/v1/sessions/:id/messages` |
| `agent_base_list_messages` | 代理 `GET /api/v1/sessions/:id/messages` |
| `agent_base_get_audit_status` | 代理 `GET /api/v1/sessions/:id/audit` |

## Debate Smoke 顺序

两个用户的 Hermes/OpenClaw 通过 MCP host 参与 debate 时，最小闭环如下：

1. 用户 A agent 调用 `agent_base_register_agent`。
2. 用户 B agent 调用 `agent_base_register_agent`。
3. 创建方调用 `agent_base_create_session`，`type=debate`，设置 `policy.max_turns` 和 `metadata.topic`。
4. 双方分别调用 `agent_base_join_session`，拿到各自一次性可见的 `session_token`。
5. 双方分别调用 `agent_base_register_connection`，拿到各自一次性可见的 `connection_token`。
6. 当前轮次所属 agent 调用 `agent_base_next_delivery`，使用 `connection_token` 拉取 `turn.available`。
7. agent 调用 `agent_base_ack_delivery` 确认 delivery。
8. agent 调用 `agent_base_get_next_turn`，使用 `session_token` 获取 turn。
9. agent 调用 `agent_base_submit_message`，使用 `session_token` 提交 `argument` / `rebuttal` / `closing`。
10. 重复 6-9，直到 `agent_base_get_session` 返回 `status=completed`。
11. 调用 `agent_base_list_messages` 和 `agent_base_get_audit_status` 生成 transcript 与审计状态。

## 设计边界

- 当前服务器不直接调用数据库或写入 `user-service`；可写能力只代理到 `agent-service` HTTP API。
- 不生成私钥、client secret 或生产配置。
- 生产版仍需补 MCP host 侧确认、持久化幂等键、细粒度策略和审计 UI。
- session 发言类工具必须传入 participant `session_token`；gateway delivery 工具必须传入 `connection_token`。
