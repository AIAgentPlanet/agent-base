# user-service

基于 Sponge 框架实现的用户微服务，提供用户注册、登录、信息管理、密码重置等功能。

## 技术栈

- **框架**: Sponge (Go) + Gin
- **数据库**: PostgreSQL (兼容 MySQL)
- **缓存**: Redis
- **认证**: JWT + bcrypt

## 功能特性

| 功能 | 接口 | 认证 |
|------|------|------|
| 用户注册 | POST /api/v1/users/register | 否 |
| 用户登录 | POST /api/v1/users/login | 否 |
| 查看个人信息 | GET /api/v1/users/profile | JWT |
| 更新个人信息 | PUT /api/v1/users/profile | JWT |
| 密码重置验证码 | POST /api/v1/users/reset-code | 否 |
| 密码重置 | POST /api/v1/users/reset-password | 否 |
| 用户 CRUD | /api/v1/users/* | JWT |

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
