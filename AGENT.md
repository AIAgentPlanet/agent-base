# Agent Base 智能体说明

本文件是当前仓库给 AI 智能体使用的总控说明。当前阶段只围绕 `user-service` 展开，用于指导智能体在改造已有网站或创建新网站时，优先复用底座中的用户、鉴权和 OAuth 能力。

## 1. 当前可用底座能力

当前仓库已落地的核心服务是：

```text
services/user-service
```

它提供以下能力：

- 用户注册
- 用户登录
- JWT 鉴权
- 当前用户资料查询与更新
- 密码重置验证码
- 密码重置
- 用户 CRUD
- OAuth Client 管理
- OAuth 授权、token、userinfo、revoke

当任务涉及用户体系、登录态、权限校验、用户资料、密码重置或 OAuth 时，智能体必须优先阅读并遵循：

```text
skills/user-service/SKILL.md
```

## 2. 智能体工作原则

1. 不要重复实现 `users` 表、`oauth_clients` 表、JWT 签发、JWT 校验、bcrypt 密码哈希、注册登录或 OAuth 流程。
2. 需要用户能力时，默认通过 `user-service` 的 HTTP API 接入。
3. 需要了解接口路径、鉴权方式、响应格式和禁止事项时，以 `skills/user-service/SKILL.md` 为准。
4. 需要修改 `user-service` 本身时，优先保持现有 Go/Sponge/Gin/GORM 分层结构。
5. 新增功能应尽量保持在现有目录边界内：`routers` 负责路由，`handler` 负责 HTTP 处理，`dao` 负责数据访问，`model` 负责数据模型，`types` 负责请求响应结构。

## 3. 任务类型判断

### 3.1 改造已有网站接入用户与 Agent 能力

当用户要求“把已有网站改造成具备 agent 能力”时，当前阶段先处理与 `user-service` 相关的用户和鉴权接入：

1. 识别现有网站是否已有用户注册、登录、鉴权、用户资料或 OAuth 实现。
2. 如果已有重复实现，优先评估迁移到 `user-service`，不要继续扩展重复用户体系。
3. 如果没有用户体系，默认接入 `user-service` 作为统一用户中心。
4. 前端登录页、注册页、资料页应调用 `user-service` API。
5. 后端受保护接口应校验来自 `user-service` 的 JWT。
6. 需要第三方应用授权时，使用 `user-service` 的 OAuth 能力。

当前阶段不要假设仓库已经存在通用 MCP、CLI 或完整 Agent Runtime。若任务需要这些能力，应先说明当前底座尚未落地，并围绕 `user-service` 完成可实现部分。

### 3.2 快速开发具备用户能力的新网站或站点

当用户要求“快速开发一个具备 agent 能力的网站或站点”时，当前阶段默认先接入 `user-service`：

1. 用户注册、登录、退出、资料页和密码重置功能全部调用 `user-service`。
2. 前端保存登录 token，并在受保护请求中携带：

```text
Authorization: Bearer <token>
```

3. 后端新增业务接口时，如果需要登录态，应校验 JWT 后再执行业务逻辑。
4. 不在新站点中创建独立用户表，除非用户明确要求并说明原因。
5. 如果新站点需要 OAuth 登录或授权能力，优先使用 `user-service` 的 OAuth 接口。

## 4. user-service 调用规则

默认服务地址：

```text
http://user-service:8080/api/v1
```

本地开发地址：

```text
http://localhost:8080/api/v1
```

常用接口：

| 场景 | 方法 | 路径 |
| --- | --- | --- |
| 注册 | `POST` | `/users/register` |
| 登录 | `POST` | `/users/login` |
| 当前用户资料 | `GET` | `/users/profile` |
| 更新当前用户资料 | `PUT` | `/users/profile` |
| 发送密码重置验证码 | `POST` | `/users/reset-code` |
| 重置密码 | `POST` | `/users/reset-password` |
| 创建 OAuth Client | `POST` | `/oauth/clients` |
| OAuth 授权 | `GET` | `/oauth/authorize` |
| OAuth 换取 token | `POST` | `/oauth/token` |
| OAuth 用户信息 | `GET` | `/oauth/userinfo` |
| OAuth 吊销 token | `POST` | `/oauth/revoke` |

完整接口说明以 `skills/user-service/SKILL.md` 和 `services/user-service/docs/swagger.yaml` 为准。

## 5. 接入交付要求

当智能体完成与 `user-service` 相关的改造或新建站点任务时，应尽量交付：

1. 登录、注册、资料、密码重置或 OAuth 的调用代码。
2. token 保存和请求头携带逻辑。
3. 受保护接口的鉴权说明或实现。
4. 必要的环境变量或配置项，例如 `USER_SERVICE_BASE_URL`。
5. README 或文档中说明如何启动 `user-service`。
6. 基础验证步骤，例如注册、登录、携带 token 访问受保护接口。

## 6. 禁止事项

- 不要在业务网站中重新设计一套用户中心。
- 不要自行实现 JWT 签发或 bcrypt 密码哈希。
- 不要绕过 `user-service` 直接操作 `users` 或 `oauth_clients` 表。
- 不要把示例 JWT secret 用于生产环境。
- 不要在未确认生产环境配置的情况下暴露 `/config` 等调试接口。

## 7. 后续扩展方向

当前 `AGENT.md` 只统筹 `user-service`。当底座继续演进时，可以再补充：

- `skills/agent-integration/SKILL.md`：指导已有网站接入完整 Agent 能力。
- `skills/agent-site/SKILL.md`：指导快速生成具备 Agent 能力的新站点。
- `mcp/`：提供 MCP Server，让智能体通过标准协议调用底座能力。
- `cli/`：提供命令行工具，用于本地、CI/CD 或脚本化调用。

在这些能力落地前，智能体应明确区分“当前已实现的 user-service 能力”和“规划中的 Agent/MCP/CLI 能力”。
