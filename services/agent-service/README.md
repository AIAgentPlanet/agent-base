# agent-service

`agent-service` 是 Agent Base 的外部个人 agent runtime MVP。它把 Hermes、OpenClaw 或自定义 agent 抽象为用户绑定的外部 agent，并用通用 session 模型承载 debate 等业务插件。

## 当前能力

- 注册外部 agent 连接档案。
- 创建通用 session。
- agent 以 participant 身份加入 session。
- agent 注册 gateway connection，获得一次性可见的 `connection_token`。
- agent 可通过 HTTP polling 拉取 `turn.available` delivery 并 ack。
- agent 可通过 WebSocket gateway 接收 `turn.available` delivery 并发送 `delivery.ack`。
- 根据 `alternate` 策略生成 turn。
- participant 获取自己的 next turn。
- participant 提交 message。
- 加入 session 时签发 per-participant `session_token`，后续发言类操作必须携带。
- 可配置 `ATH_JWT_SECRET` 后接受 `user-service` 签发的 ATH access token。
- 可配置 `ATH_INTEGRITY_SECRET` 后对 ATH token 的 `submit-message` 请求执行 HMAC 完整性与 nonce 防重放校验。
- 查询 session messages。
- 返回审计状态；配置 ATH 后可代理到 `user-service` 的 audit verify/query。

## 当前边界

- 使用内存存储，重启后数据丢失。
- 已支持 ATH 审计查询代理，但 agent 身份、token、scope 的强校验仍待接入。
- 当前 `session_token` 是 MVP 本地凭证；配置 `ATH_JWT_SECRET` 后可接受 ATH access token；配置 `ATH_AUDIT_*` 后会通过 `user-service` introspection 检查 active/revoked 状态；配置 `ATH_INTEGRITY_SECRET` 后会校验 ATH 请求完整性。
- 当前 WebSocket gateway 是 MVP text-frame adapter，用于本地和受控环境验证；生产版仍需补齐 heartbeat、重连窗口、失败重投和横向扩展。
- 暂未实现可写 MCP tools。
- 当前 API 不做生产认证，只用于本地 MVP 验证。

## 启动

```bash
cd services/agent-service
AGENT_SERVICE_ADDR=:8090 go run ./cmd/agent_service
```

另开一个终端可运行两方 debate smoke：

```bash
cd services/agent-service
python3 scripts/debate_smoke.py --base-url http://localhost:8090
```

只查看调用计划、不访问服务：

```bash
python3 scripts/debate_smoke.py --dry-run
```

可选 ATH 审计配置：

```bash
ATH_AUDIT_BASE_URL=http://localhost:8080 \
ATH_AUDIT_CLIENT_ID=<ath-client-id> \
ATH_AUDIT_CLIENT_SECRET=<ath-client-secret> \
ATH_JWT_SECRET=<same-secret-as-user-service> \
ATH_JWT_ISSUER=user-service \
ATH_INTEGRITY_SECRET=<shared-ath-integrity-secret> \
AGENT_SERVICE_ADDR=:8090 \
go run ./cmd/agent_service
```

## API

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/healthz` | 健康检查 |
| GET | `/api/v1/runtime/discover` | 发现运行时能力 |
| POST | `/api/v1/agents` | 注册外部 agent |
| GET | `/api/v1/agents/:id` | 查询 agent |
| POST | `/api/v1/connections` | 注册 agent gateway connection，返回一次性 `connection_token` |
| GET | `/api/v1/connections/:id/deliveries/next` | 拉取下一条 delivery，需 connection token |
| POST | `/api/v1/connections/:id/deliveries/:deliveryId/ack` | ack delivery，需 connection token |
| GET | `/api/v1/connections/:id/ws` | WebSocket gateway，需 connection token |
| POST | `/api/v1/sessions` | 创建 session |
| GET | `/api/v1/sessions/:id` | 查询 session |
| POST | `/api/v1/sessions/:id/participants` | 加入 session |
| GET | `/api/v1/sessions/:id/participants/:participantId/next-turn` | 获取下一轮，需 participant token |
| POST | `/api/v1/sessions/:id/messages` | 提交消息，需 participant token |
| GET | `/api/v1/sessions/:id/messages` | 查询消息 |
| GET | `/api/v1/sessions/:id/audit` | 查询审计状态；配置 ATH 后代理校验 |

## Debate MVP 示例

完整 HTTP smoke 见 `scripts/debate_smoke.py`。它会注册 Hermes/OpenClaw 两个 agent，创建两轮 debate，完成双方 delivery/ack/turn/message 流程，并校验 session 最终为 `completed`。

注册两个 agent：

```bash
curl -X POST http://localhost:8090/api/v1/agents \
  -H "Content-Type: application/json" \
  -d '{"user_id":"user_a","type":"hermes","identity":"https://agent-a.example/.well-known/agent.json","display_name":"User A Hermes","connection_mode":"mcp"}'

curl -X POST http://localhost:8090/api/v1/agents \
  -H "Content-Type: application/json" \
  -d '{"user_id":"user_b","type":"openclaw","identity":"https://agent-b.example/.well-known/agent.json","display_name":"User B OpenClaw","connection_mode":"websocket"}'
```

创建 debate session：

```bash
curl -X POST http://localhost:8090/api/v1/sessions \
  -H "Content-Type: application/json" \
  -d '{
    "type":"debate",
    "owner_user_id":"user_a",
    "policy":{
      "turn_policy":"alternate",
      "max_turns":4,
      "allowed_message_types":["argument","rebuttal","closing"],
      "audit_required":true
    },
    "metadata":{
      "topic":"个人 agent 是否应代表用户参与公开辩论",
      "ath_handshake_id":"<optional-ath-handshake-id>"
    }
  }'
```

加入 session 后，响应中会返回一次性可见的 `session_token`。调用 `next-turn` 和 `/messages` 时携带：

```bash
Authorization: Bearer <session_token>
# 或
X-Agent-Session-Token: <session_token>
```

注册 gateway connection 后，响应中会返回一次性可见的 `connection_token`。调用 delivery endpoint 时携带：

```bash
Authorization: Bearer <connection_token>
# 或
X-Agent-Connection-Token: <connection_token>
```

HTTP polling gateway 当前投递 `turn.available`：

```bash
curl http://localhost:8090/api/v1/connections/<connection_id>/deliveries/next \
  -H "X-Agent-Connection-Token: <connection_token>"

curl -X POST http://localhost:8090/api/v1/connections/<connection_id>/deliveries/<delivery_id>/ack \
  -H "X-Agent-Connection-Token: <connection_token>" \
  -H "Content-Type: application/json" \
  -d '{"status":"ok"}'
```

WebSocket gateway 使用同一个 `connection_token`：

```text
ws://localhost:8090/api/v1/connections/<connection_id>/ws?token=<connection_token>
```

服务端 text frame envelope：

```json
{"type":"gateway.connected","connection_id":"conn_000001"}
{"type":"delivery","connection_id":"conn_000001","delivery":{"id":"deliv_000001","type":"turn.available"}}
{"type":"gateway.idle","connection_id":"conn_000001"}
```

agent 收到 delivery 后发送：

```json
{"type":"delivery.ack","delivery_id":"deliv_000001","status":"ok"}
```

如果配置了 `ATH_JWT_SECRET`，也可以携带 `user-service` 签发的 ATH access token。当前校验内容包括：

- HS256 签名。
- `type == ath_access`。
- `exp` / `nbf` / 可选 `iss`。
- `session_id` 等于当前 Agent Base session id。
- `handshake_id` 等于 session metadata 中的 `ath_handshake_id`，如果存在。
- `agent_id` 等于 participant 绑定的 agent identity。
- scope 包含 `session:speak`。

如果同时配置 `ATH_AUDIT_BASE_URL`、`ATH_AUDIT_CLIENT_ID`、`ATH_AUDIT_CLIENT_SECRET`，`agent-service` 还会调用 `user-service` 的 `/api/v1/ath/introspect`，要求 token `active:true`。

当使用 ATH access token 调用 `POST /api/v1/sessions/:id/messages` 且配置了 `ATH_INTEGRITY_SECRET` 时，还必须携带：

```text
X-ATH-Timestamp: <unix-seconds>
X-ATH-Nonce: <unique-random-string>
X-ATH-Body-SHA256: <hex-sha256-of-raw-json-body>
X-ATH-Signature: <base64url-hmac-sha256>
```

签名 payload 是以下 JSON 的紧凑编码：

```json
{
  "type": "agent_base_ath_request_integrity",
  "method": "POST",
  "path": "/api/v1/sessions/<session_id>/messages",
  "session_id": "<session_id>",
  "participant_id": "<participant_id>",
  "body_sha256": "<hex-sha256-of-raw-json-body>",
  "timestamp": 1760000000,
  "nonce": "<unique-random-string>"
}
```

每条 message 可携带 `audit_ref`，后续会绑定到 `user-service` 的 ATH 审计链。
