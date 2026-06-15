# user-service

基于 Sponge 框架实现的用户微服务，提供用户注册、登录、信息管理、密码重置等功能。

## 技术栈

- **框架**: Sponge (Go) + Gin
- **数据库**: PostgreSQL (兼容 MySQL)
- **缓存**: Redis
- **认证**: JWT + bcrypt
- **AI 可信交互**: ATH (Agent Trust Handshake) 协议

## 功能特性

### 用户服务

| 功能 | 接口 | 认证 |
|------|------|------|
| 用户注册 | POST /api/v1/users/register | 否 |
| 用户登录 | POST /api/v1/users/login | 否 |
| 查看个人信息 | GET /api/v1/users/profile | JWT |
| 更新个人信息 | PUT /api/v1/users/profile | JWT |
| 密码重置验证码 | POST /api/v1/users/reset-code | 否 |
| 密码重置 | POST /api/v1/users/reset-password | 否 |
| 用户 CRUD | /api/v1/users/* | JWT |

### OAuth 2.0

| 功能 | 接口 | 认证 |
|------|------|------|
| 授权端点 | GET /api/v1/oauth/authorize | JWT |
| Token 端点 | POST /api/v1/oauth/token | 否 |
| 用户信息 | GET /api/v1/oauth/userinfo | OAuth Bearer |
| Token 注销 | POST /api/v1/oauth/revoke | 否 |
| 客户端管理 | /api/v1/oauth/clients/* | JWT |

### ATH 协议 (Agent Trust Handshake)

支持 AI Agent 通过可信握手协议安全访问用户服务。

| 功能 | 接口 | 认证 |
|------|------|------|
| 发现文档 | GET /.well-known/ath.json | 否 |
| 服务端 DID 文档 | GET /.well-known/did.json | 否 |
| Agent 注册 | POST /api/v1/ath/agents/register | Attestation JWT |
| Agent 状态 | GET /api/v1/ath/agents/:clientId | Attestation JWT |
| 发起身份握手 | POST /api/v1/ath/handshakes | 已注册 Agent |
| 提交身份签名 | POST /api/v1/ath/handshakes/:handshakeId/proof | ES256 签名 |
| 握手状态 | GET /api/v1/ath/handshakes/:handshakeId | Client ID |
| 用户授权 | POST /api/v1/ath/authorize | Attestation JWT |
| Token 交换 | POST /api/v1/ath/token | Client Secret |
| Token 注销 | POST /api/v1/ath/revoke | Client Secret |
| API 代理 | POST /api/v1/ath/proxy | ATH Bearer |
| 审计记录查询 | POST /api/v1/ath/audit/query | Client Secret |
| 审计链校验 | POST /api/v1/ath/audit/verify | Client Secret |
| 公开审计链头 | GET /.well-known/ath-audit-head.json | 否 |
| 锚定状态 | POST /api/v1/ath/audit/anchor/status | Client Secret |
| 锚定重试 | POST /api/v1/ath/audit/anchor/retry | Client Secret |

## 快速开始

### 1. 启动依赖服务

```bash
cd deployments
docker-compose up -d postgres redis
```

### 2. 运行服务

```bash
# 本地运行
go run cmd/user_service/main.go -c configs/user_service.yml

# 或使用 Makefile
make run
```

### 3. 访问 API 文档

启动后访问: http://localhost:8080/swagger/index.html

## 配置说明

配置文件: `configs/user_service.yml`

```yaml
# 数据库
database:
  driver: "postgresql"
  postgresql:
    dsn: "postgres://postgres:postgres@127.0.0.1:5432/user_service?sslmode=disable"

# Redis
redis:
  dsn: "default:@127.0.0.1:6379/0"

# JWT
jwt:
  secret: "your-secret-key"
  issuer: "user-service"
  expireHours: 24
```

## API 示例

### 注册
```bash
curl -X POST http://localhost:8080/api/v1/users/register \
  -H "Content-Type: application/json" \
  -d '{"username":"test","password":"123456","email":"test@example.com","phone":"13800138000"}'
```

### 登录
```bash
curl -X POST http://localhost:8080/api/v1/users/login \
  -H "Content-Type: application/json" \
  -d '{"username":"test","password":"123456"}'
```

### 查看个人信息
```bash
curl -X GET http://localhost:8080/api/v1/users/profile \
  -H "Authorization: Bearer <token>"
```

## ATH 协议使用指南

### 什么是 ATH

本服务实现 ATH v0.1 网关模式的 M5 密钥轮换与外部锚定能力，基于 OAuth 2.0 扩展并增加：
- **Agent 身份验证**: ES256 签名 Attestation JWT
- **双向身份验证**: 服务端挑战签名 + Agent 身份证明签名
- **握手状态机**: `challenge_issued` 到 `identity_verified` 的原子迁移
- **会话密钥**: P-256 ECDH + HKDF-SHA256 派生 256 位短期密钥
- **请求完整性**: HMAC-SHA256 绑定 Token、方法、路径、正文摘要和握手
- **重放保护**: 请求时间窗与 Redis 原子一次性 nonce
- **审计哈希链**: 全局连续序号、前序哈希与 SHA-256 记录哈希
- **签名存证**: 每条记录使用网关 ES256 身份密钥签名
- **数据库追加保护**: PostgreSQL 触发器拒绝审计记录的更新和删除
- **外部锚点**: 公开最新签名链头，支持监控系统定期留存
- **签名密钥环**: active key 负责新签名，历史公钥继续通过 DID 文档发布
- **远程 KMS 签名**: 支持 HTTPS 摘要签名服务，业务进程无需持有私钥
- **Transactional Outbox**: 审计记录和待投递锚点在同一数据库事务写入
- **可靠投递**: 多实例安全领取、幂等键、指数退避及崩溃锁恢复
- **最小权限原则**: 按 scope 精细控制访问权限
- **身份文档**: 支持 `did:web` 与公共 HTTPS Agent Identity Document，并发布服务端 DID 文档
- **授权绑定**: OAuth 会话与 ATH Token 显式绑定已验证的 `handshake_id`

本地审计链能够检测数据库内容篡改，但数据库管理员仍可能删除触发器或整体回滚数据库。
生产环境应将 `/.well-known/ath-audit-head.json` 定期提交到独立日志系统、
可信时间戳服务或区块链，形成真正独立的外部锚点。

### M5 密钥轮换

生产环境推荐使用密钥环配置：

```yaml
ath:
  serverDID: "did:web:gateway.example.com"
  activeSigningKeyID: "did:web:gateway.example.com#key-2026"
  signingKeys:
    - id: "did:web:gateway.example.com#key-2025"
      keyFile: "/run/secrets/ath-key-2025.pem"
    - id: "did:web:gateway.example.com#key-2026"
      keyFile: "/run/secrets/ath-key-2026.pem"
```

轮换时先添加新密钥并保持旧密钥，再将 `activeSigningKeyID` 指向新密钥并重启服务。
旧公钥应至少保留到所有历史签名的验证保留期结束。

远程 KMS 网关配置：

```yaml
signingKeys:
  - id: "did:web:gateway.example.com#key-kms-2026"
    publicKeyFile: "/etc/ath/key-kms-2026-public.pem"
    signingEndpoint: "https://kms-gateway.example.com/v1/sign"
    authToken: "<secret>"
```

服务向 KMS 网关提交：

```json
{"key_id":"did:web:gateway.example.com#key-kms-2026","algorithm":"ES256","digest":"<base64url-sha256>"}
```

网关返回 `{"signature":"<base64url-ecdsa-asn1-signature>"}`。服务会使用已发布公钥复验签名，
错误或不匹配的签名不会进入握手或审计链。

### M5 外部锚定

```yaml
ath:
  anchor:
    endpoint: "https://anchor.example.com/v1/ath/events"
    authToken: "<secret>"
    intervalSeconds: 30
    batchSize: 50
    timeoutSeconds: 10
```

Webhook 请求使用 `event_id` 作为 `Idempotency-Key`，并携带
`X-ATH-Sequence` 与 `X-ATH-Record-Hash`。接收端必须按幂等键去重。
投递失败会按 1、2、4、8 分钟递增，最大退避 128 分钟。

Agent Identity Document 必须通过公共 HTTPS 发布，并包含与 Attestation
私钥匹配的 P-256 PEM 或 JWK 公钥。服务端会验证 `iss`、`sub`、`aud`、
`iat`、`exp` 和 `jti`，并拒绝重放。

### ATH 完整流程

```
1. Discovery    → GET  /.well-known/ath.json
2. Register     → POST /api/v1/ath/agents/register  (提交 Attestation JWT)
3. Challenge    → POST /api/v1/ath/handshakes       (获取服务端签名挑战)
4. Client Proof → POST /api/v1/ath/handshakes/:id/proof (提交 Agent ES256 签名)
5. Authorize    → POST /api/v1/ath/authorize        (绑定 handshake_id)
6. User Consent → GET  /api/v1/oauth/authorize      (用户登录并授权)
7. Token        → POST /api/v1/ath/token            (用 code 换 ATH token)
8. API Call     → POST /api/v1/ath/proxy            (用 ATH token 调用 API)
9. Revoke       → POST /api/v1/ath/revoke           (注销 token)
10. Audit       → POST /api/v1/ath/audit/query       (查询握手审计记录)
11. Verify      → POST /api/v1/ath/audit/verify      (验证完整哈希链)
```

### ATH 快速示例

#### 1. 生成 ES256 密钥对

```bash
openssl ecparam -genkey -name prime256v1 -noout -out agent-private.pem
openssl ec -in agent-private.pem -pubout -out agent-public.pem
```

#### 2. Agent 注册

```bash
# 先生成 attestation JWT (Python 示例)
python3 -c "
import jwt, time, uuid
from cryptography.hazmat.primitives import serialization
with open('agent-private.pem', 'rb') as f:
    private_key = serialization.load_pem_private_key(f.read(), password=None)
claims = {
    'iss': 'https://agent.example.com/.well-known/agent.json',
    'sub': 'https://agent.example.com/.well-known/agent.json',
    'aud': 'http://127.0.0.1:8080/api/v1/ath/agents/register',
    'iat': int(time.time()),
    'exp': int(time.time()) + 300,
    'jti': str(uuid.uuid4()),
}
print(jwt.encode(claims, private_key, algorithm='ES256'))
" > attestation.jwt

# 注册 Agent
curl -X POST http://localhost:8080/api/v1/ath/agents/register \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "https://agent.example.com/.well-known/agent.json",
    "agent_attestation": "'$(cat attestation.jwt)'",
    "name": "My AI Agent",
    "developer": {"name": "My Org", "id": "dev-001"},
    "redirect_uris": ["http://localhost:3000/callback"],
    "requested_providers": [{"provider_id": "user-service", "scopes": ["user:read"]}]
  }'
```

注册成功后返回 `client_id` 和 `client_secret`，后续步骤需要用到。

#### 3. 完成双向身份握手

Agent 先提交至少 256 位随机数、P-256 临时公钥、支持版本和能力。服务端返回
自己的临时公钥和签名挑战，双方使用 ECDH 共享秘密并通过 HKDF-SHA256
派生 32 字节会话密钥。HKDF 参数如下：

- `salt = SHA256(base64url_decode(client_nonce) || base64url_decode(server_nonce))`
- `info` 为紧凑 JSON：`protocol`、`handshake_id`、`client_did`、`server_did`、`version`
- `protocol` 固定为 `ATH-ECDH-P256-HKDF-SHA256`

随后
Agent 使用注册时对应的 ES256 私钥签名以下紧凑 JSON：

```json
{"type":"client_identity_proof","handshake_id":"<handshake_id>","client_did":"<agent_id>","server_did":"<server_did>","server_nonce":"<server_nonce>","client_ephemeral_key":"<client-public-key>","server_ephemeral_key":"<server-public-key>","version":"0.1","timestamp":<unix_seconds>}
```

签名采用 ECDSA ASN.1 DER 格式，并使用无填充 Base64URL 编码。服务端挑战使用
同样的编码规则，客户端应通过 `/.well-known/did.json` 或对应 `did:web`
文档验证服务端签名。

#### 4. 获取用户授权

```bash
curl -X POST http://localhost:8080/api/v1/ath/authorize \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "<client_id>",
    "handshake_id": "<verified-handshake-id>",
    "agent_attestation": "<fresh-attestation-for-authorize-endpoint>",
    "provider_id": "user-service",
    "scopes": ["user:read"],
    "state": "<at-least-128-bits-of-random-state>",
    "user_redirect_uri": "http://localhost:3000/callback"
  }'
```

返回 `authorization_url` 和 `ath_session_id`。用户访问 `authorization_url` 完成登录和授权后，会携带 `code` 重定向回来。

#### 5. 交换 Token

```bash
curl -X POST http://localhost:8080/api/v1/ath/token \
  -H "Content-Type: application/json" \
  -d '{
    "grant_type": "authorization_code",
    "code": "<authorization-code>",
    "client_id": "<client_id>",
    "client_secret": "<client_secret>",
    "agent_attestation": "<fresh-attestation-for-token-endpoint>",
    "ath_session_id": "<ath_session_id>",
  }'
```

返回 `access_token` 和 `refresh_token`。

#### 6. 调用受保护 API

```bash
curl -X POST http://localhost:8080/api/v1/ath/proxy \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <ath-access-token>" \
  -d '{
    "provider": "user-service",
    "method": "GET",
    "path": "/api/v1/users/profile",
    "request_timestamp": 1750000000,
    "request_nonce": "<base64url-128-bit-random>",
    "request_signature": "<base64url-hmac-sha256>"
  }'
```

`request_signature` 使用会话密钥对以下紧凑 JSON 做 HMAC-SHA256：

```json
{"type":"ath_request_integrity","handshake_id":"<handshake_id>","token_jti":"<access-token-jti>","provider":"user-service","method":"GET","path":"/api/v1/users/profile","body_sha256":"<lowercase-hex-sha256>","timestamp":1750000000,"nonce":"<request_nonce>"}
```

#### 7. 查询和验证存证

```bash
curl -X POST http://localhost:8080/api/v1/ath/audit/query \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "<client_id>",
    "client_secret": "<client_secret>",
    "handshake_id": "<handshake_id>",
    "limit": 100
  }'

curl -X POST http://localhost:8080/api/v1/ath/audit/verify \
  -H "Content-Type: application/json" \
  -d '{
    "client_id": "<client_id>",
    "client_secret": "<client_secret>"
  }'

curl http://localhost:8080/.well-known/ath-audit-head.json
```

查询 outbox 状态或手动重新入队：

```bash
curl -X POST http://localhost:8080/api/v1/ath/audit/anchor/status \
  -H "Content-Type: application/json" \
  -d '{"client_id":"<client_id>","client_secret":"<client_secret>"}'

curl -X POST http://localhost:8080/api/v1/ath/audit/anchor/retry \
  -H "Content-Type: application/json" \
  -d '{"client_id":"<client_id>","client_secret":"<client_secret>"}'
```

审计载荷不会保存访问令牌、客户端密钥、会话密钥或代理请求正文。代理调用只记录正文摘要。
成功写入审计链的业务响应会包含 `X-ATH-Audit-Status: recorded`、
`X-ATH-Audit-Event-ID` 和 `X-ATH-Audit-Record-Hash`；写入失败时状态为 `failed`，
调用方应触发告警。

### 自动化测试

项目提供了完整的 ATH 端到端测试脚本：

```bash
AGENT_ID=https://agent.example.com/.well-known/agent.json \
AGENT_PRIVATE_KEY=agent-private.pem \
./scripts/test-ath.sh
```

该脚本会自动完成 Discovery → Register → Authorize → Token → Proxy → Revoke 的完整流程。
