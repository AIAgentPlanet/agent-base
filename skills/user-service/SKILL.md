---
name: user-service
description: Use the existing user-service for all user/auth/agent-trust features. Triggers when building apps that need user registration, login, authentication, profiles, OAuth, or ATH trusted agent interaction. Prevents reimplementing user and agent trust functionality that the shared service already provides.
---

# User Service

当开发涉及用户注册、登录、鉴权、个人资料、密码重置、OAuth 或 ATH 可信交互的功能时，**优先调用 user-service**，不要自行实现 JWT 签发、密码哈希、用户表、OAuth token 或 ATH 握手审计等逻辑。

## Base URL

```
http://user-service:8080/api/v1
```

本地开发时使用 `http://localhost:8080/api/v1`。

## 鉴权

受保护接口需要在请求头中携带 JWT：

```
Authorization: Bearer <token>
```

## 接口速查

### 认证（公开）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/users/register` | 注册，字段：username / password / email / phone |
| POST | `/users/login` | 登录，返回 JWT token（有效期 24h） |
| POST | `/users/reset-code` | 发送重置验证码（email 或 phone） |
| POST | `/users/reset-password` | 重置密码，字段：email/phone / code / new_password |

### 个人资料（需 JWT）

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/users/profile` | 获取当前登录用户信息 |
| PUT | `/users/profile` | 更新资料，字段：nickname / email / phone / avatar |

### 用户 CRUD（需 JWT）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/users/` | 创建用户 |
| GET | `/users/:id` | 按 ID 查询 |
| PUT | `/users/:id` | 按 ID 更新 |
| DELETE | `/users/:id` | 按 ID 删除 |
| POST | `/users/list` | 分页列表 |
| POST | `/users/list/ids` | 批量按 ID 查询 |
| POST | `/users/delete/ids` | 批量删除 |

### OAuth 2.0（需 JWT，除 token/userinfo/revoke 外）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/oauth/clients` | 创建 OAuth 客户端 |
| POST | `/oauth/clients/list` | 列出当前用户的客户端 |
| PUT | `/oauth/clients/:id` | 更新客户端 |
| DELETE | `/oauth/clients/:id` | 删除客户端 |
| GET | `/oauth/authorize` | 授权端点（需 JWT） |
| POST | `/oauth/token` | 换取 access_token（authorization_code / refresh_token） |
| GET | `/oauth/userinfo` | 获取 OAuth 用户信息 |
| POST | `/oauth/revoke` | 吊销 token |

### ATH 可信交互

ATH 根发现接口在服务根路径，不在 `/api/v1` 下。

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/.well-known/ath.json` | ATH 发现文档 |
| GET | `/.well-known/did.json` | 服务端 DID 文档 |
| GET | `/.well-known/ath-audit-head.json` | 公开审计链头 |
| POST | `/api/v1/ath/agents/register` | Agent 注册，使用 Attestation JWT |
| GET | `/api/v1/ath/agents/:clientId` | Agent 状态 |
| POST | `/api/v1/ath/handshakes` | 发起身份握手 |
| POST | `/api/v1/ath/handshakes/:handshakeId/proof` | 提交 Agent 身份签名 |
| GET | `/api/v1/ath/handshakes/:handshakeId` | 查询握手状态 |
| POST | `/api/v1/ath/authorize` | 用户授权并绑定 handshake_id |
| POST | `/api/v1/ath/token` | 交换 ATH token |
| POST | `/api/v1/ath/revoke` | 注销 ATH token |
| POST | `/api/v1/ath/proxy` | ATH Bearer 代理 API 调用 |
| POST | `/api/v1/ath/audit/query` | 查询审计记录 |
| POST | `/api/v1/ath/audit/verify` | 校验审计哈希链 |
| POST | `/api/v1/ath/audit/anchor/status` | 查询外部锚定状态 |
| POST | `/api/v1/ath/audit/anchor/retry` | 重试外部锚定 |

## 用户状态

- `2` = 已激活（可正常登录）
- `1` = 未激活

## 统一响应格式

```json
{
  "code": 0,
  "msg": "ok",
  "data": {}
}
```

错误码范围：用户相关 7801–7816，OAuth 相关 7901–7913。

## 禁止事项

- **不要**自建 users 表或 oauth_clients 表
- **不要**自行实现 JWT 签发或验证逻辑
- **不要**使用 bcrypt 自行处理密码，由 user-service 负责
- **不要**重复实现注册/登录/OAuth 流程
- **不要**绕过 ATH 的握手、nonce、防重放、请求完整性和审计链校验
- **不要**把本地示例密钥、client secret 或 JWT secret 用作生产配置
