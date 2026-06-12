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
| Agent 注册 | POST /api/v1/ath/agents/register | Attestation JWT |
| Agent 状态 | GET /api/v1/ath/agents/:clientId | Attestation JWT |
| 用户授权 | POST /api/v1/ath/authorize | Attestation JWT |
| Token 交换 | POST /api/v1/ath/token | Client Secret |
| Token 注销 | POST /api/v1/ath/revoke | Client Secret |
| API 代理 | POST /api/v1/ath/proxy | ATH Bearer |

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

ATH (Agent Trust Handshake) 是专为 AI Agent 设计的可信交互协议，基于 OAuth 2.0 扩展，增加了：
- **Agent 身份验证**: ES256 签名 Attestation JWT
- **双向可信握手**: 用户授权 + 服务授权双重确认
- **最小权限原则**: 按 scope 精细控制访问权限
- **全程可追溯**: 所有交互都有签名存证

### ATH 完整流程

```
1. Discovery    → GET  /.well-known/ath.json
2. Register     → POST /api/v1/ath/agents/register  (提交 Attestation JWT)
3. Authorize    → POST /api/v1/ath/authorize        (获取用户授权 URL)
4. User Consent → GET  /api/v1/oauth/authorize      (用户登录并授权)
5. Token        → POST /api/v1/ath/token            (用 code 换 ATH token)
6. API Call     → POST /api/v1/ath/proxy            (用 ATH token 调用 API)
7. Revoke       → POST /api/v1/ath/revoke           (注销 token)
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
    'iss': 'did:ath:my-agent',
    'sub': 'did:ath:my-agent',
    'aud': 'user-service',
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
    "agent_id": "did:ath:my-agent",
    "attestation": "'$(cat attestation.jwt)'",
    "name": "My AI Agent",
    "developer": {"name": "My Org", "id": "dev-001"},
    "redirect_uris": ["http://localhost:3000/callback"],
    "providers": [{"provider_id": "user-service", "scopes": ["user:read"]}]
  }'
```

注册成功后返回 `client_id` 和 `client_secret`，后续步骤需要用到。

#### 3. 获取用户授权

```bash
curl -X POST http://localhost:8080/api/v1/ath/authorize \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <attestation-jwt>" \
  -d '{
    "client_id": "<client_id>",
    "provider_id": "user-service",
    "scopes": ["user:read"],
    "state": "xyz123",
    "redirect_uri": "http://localhost:3000/callback"
  }'
```

返回 `authorization_url` 和 `ath_session_id`。用户访问 `authorization_url` 完成登录和授权后，会携带 `code` 重定向回来。

#### 4. 交换 Token

```bash
curl -X POST http://localhost:8080/api/v1/ath/token \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <attestation-jwt>" \
  -d '{
    "grant_type": "authorization_code",
    "code": "<authorization-code>",
    "client_id": "<client_id>",
    "client_secret": "<client_secret>",
    "ath_session_id": "<ath_session_id>",
    "redirect_uri": "http://localhost:3000/callback"
  }'
```

返回 `access_token` 和 `refresh_token`。

#### 5. 调用受保护 API

```bash
curl -X POST http://localhost:8080/api/v1/ath/proxy \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <ath-access-token>" \
  -d '{
    "provider": "user-service",
    "method": "GET",
    "path": "/api/v1/users/profile"
  }'
```

### 自动化测试

项目提供了完整的 ATH 端到端测试脚本：

```bash
chmod +x scripts/test-ath.sh
./scripts/test-ath.sh
```

该脚本会自动完成 Discovery → Register → Authorize → Token → Proxy → Revoke 的完整流程。
