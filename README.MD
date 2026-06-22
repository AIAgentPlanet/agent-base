# Agent Base

> AI 时代 Web 应用开发底座框架

Agent Base 是一个面向 AI 原生开发的 Web 应用底座框架，提供开箱即用的提示词库、MCP 服务、CLI 工具和标准化微服务模板，帮助开发者在 AI 协作模式下快速构建和部署 Web 应用。

---

## 为什么需要 Agent Base？

在 AI 辅助开发的工作流中，开发者面临三个核心挑战：

1. **如何让 AI 更好地理解你的业务** — 需要高质量、可复用的提示词和技能定义
2. **如何让 AI Agent 直接调用应用能力** — 需要标准化的 MCP 接口和 CLI 工具
3. **如何让 AI 基于统一代码规范生成代码** — 需要一套经过验证的基础代码库

Agent Base 为这三个问题提供了系统性解答。

---

## 项目结构

```
agent-base/
├── prompts/          # 提示词库
├── skills/           # AI 技能定义
├── mcp/              # MCP Server 实现
├── cli/              # CLI 工具
├── specs/            # 通用 Agent Runtime 契约
└── services/         # 基础微服务模板
    ├── user-service  # 用户服务（注册/登录/鉴权）
    └── agent-service # 外部 agent runtime MVP
```

---

## 核心模块

### Prompts & Skills

提供面向业务场景的提示词库和 AI 技能定义：

- 覆盖既有网站接入、新站点开发、ATH 可信交互等场景的提示词模板
- 标准化 Skill 接口，可直接集成到 Codex、Claude Code、Cursor 等 AI 开发工具
- 支持团队自定义扩展，沉淀业务领域知识

当前内置：

| 类型 | 文件 | 说明 |
|------|------|------|
| Skill | `skills/agent-base/SKILL.md` | Agent Base 框架、prompt/skill、CLI/MCP 使用规则 |
| Skill | `skills/user-service/SKILL.md` | 用户、鉴权、OAuth、ATH 接入规则 |
| Prompt | `prompts/site-integration.md` | 既有网站接入 Agent Base |
| Prompt | `prompts/new-agent-site.md` | 新建 AI 原生站点 |
| Prompt | `prompts/ath-integration.md` | ATH 可信交互接入 |
| Prompt | `prompts/external-agent-session.md` | Hermes/OpenClaw 等外部个人 agent 接入通用 session |

### MCP Server & CLI

让 AI Agent 直接调用应用能力：

- 基于 [Model Context Protocol (MCP)](https://modelcontextprotocol.io) 标准实现
- 提供 CLI 工具，支持在终端和 CI/CD 流程中调用
- 低侵入接入，不依赖特定 AI 平台

当前提供轻量 MCP/CLI 工具层：

```bash
# 检查框架关键文件
python3 cli/agent-base doctor

# 输出 Agent Base 上下文
python3 cli/agent-base context --format markdown

# 查看 prompt 和 skill
python3 cli/agent-base prompts list
python3 cli/agent-base skills list

# 查看 user-service / ATH 接口清单
python3 cli/agent-base user-service endpoints
python3 cli/agent-base user-service ath-endpoints

# 查看通用外部 agent runtime 契约
python3 cli/agent-base runtime connection-modes
python3 cli/agent-base runtime session-types
python3 cli/agent-base runtime tools

# 启动 MCP stdio server
python3 mcp/agent_base_mcp.py
```

MCP 当前暴露 `agent_base_context`、`list_prompts`、`read_prompt`、`list_skills`、`read_skill`、`user_service_endpoints`、`ath_endpoints`、`agent_runtime_discover`、`list_connection_modes`、`list_session_types`、`list_runtime_tools` 和 `read_runtime_spec` 等只读工具，也暴露 `agent_base_register_agent`、`agent_base_create_session`、`agent_base_join_session`、`agent_base_get_session`、`agent_base_register_connection`、`agent_base_next_delivery`、`agent_base_ack_delivery`、`agent_base_get_next_turn`、`agent_base_submit_message`、`agent_base_list_messages`、`agent_base_get_audit_status` 等 `agent-service` proxy tools。完整 Agent Runtime、长期任务调度、生产级 MCP 确认/幂等/审计仍属于后续扩展。

### 通用 Agent Runtime 契约

`specs/agent-runtime/` 定义外部个人 agent 接入 Agent Base 的通用契约，适用于 Hermes、OpenClaw 或自定义 agent：

| 文件 | 说明 |
|------|------|
| `README.md` | 通用分层、核心概念和安全原则 |
| `connection-gateway.md` | HTTPS webhook、WebSocket、MCP 和 HTTP polling MVP 连接模式 |
| `mcp-tools.md` | 通用 MCP tools 契约 |
| `session-schema.json` | Session、Participant、Turn、Message 的基础数据模型 |
| `debate-plugin.md` | 辩论作为第一个业务插件示例 |

### 基础代码库 (Services)

经过验证的微服务模板，AI 基于此生成代码时风格一致、质量可控：

| 模块 | 技术栈 | 说明 |
|------|--------|------|
| user-service | Go · Gin · PostgreSQL · Redis · JWT · OAuth2 · ATH | 用户注册、登录、信息管理、OAuth、Agent 可信交互 |
| agent-service | Go · net/http · In-memory | 外部 agent 注册、通用 session、debate 轮次调度 MVP |

**技术选型**

- 框架：[Sponge](https://github.com/zhufuyi/sponge) + Gin
- 数据库：PostgreSQL（兼容 MySQL）
- 缓存：Redis
- 认证：JWT + bcrypt + OAuth2
- Agent 可信交互：ATH（Agent Trust Handshake）
- 文档：Swagger / OpenAPI

---

## 快速开始

### 前置依赖

- Go 1.21+
- Docker & Docker Compose

### 启动 user-service

```bash
# 1. 启动基础设施
cd services/user-service/deployments
docker-compose up -d postgres redis

# 2. 运行服务
cd ..
go run cmd/user_service/main.go -c configs/user_service.yml

# 3. 访问 API 文档
open http://localhost:8080/swagger/index.html
```

### 启动 agent-service MVP

`agent-service` 当前是内存版 MVP，用来验证通用 agent/session/orchestrator 抽象。它已支持 agent gateway connection 注册、HTTP polling delivery 拉取和 ack、WebSocket delivery/ack，支持 per-participant `session_token` 保护 `next-turn` 和 `submit-message`，也可配置 `ATH_JWT_SECRET` 接受 `user-service` 签发的 ATH access token，可选代理到 `user-service` 的 ATH introspection 与 audit verify/query，并支持 `ATH_INTEGRITY_SECRET` 校验 `submit-message` 的 HMAC 完整性与 nonce 防重放；但还没有生产持久化或可写 MCP tools。

```bash
cd services/agent-service
AGENT_SERVICE_ADDR=:8090 go run ./cmd/agent_service

# 发现能力
curl http://localhost:8090/api/v1/runtime/discover
```

如需让 `/api/v1/sessions/:id/audit` 代理到 `user-service` 的 ATH 审计接口，可配置：

```bash
ATH_AUDIT_BASE_URL=http://localhost:8080
ATH_AUDIT_CLIENT_ID=<ath-client-id>
ATH_AUDIT_CLIENT_SECRET=<ath-client-secret>
ATH_JWT_SECRET=<same-secret-as-user-service>
ATH_JWT_ISSUER=user-service
ATH_INTEGRITY_SECRET=<shared-ath-integrity-secret>
```

### 使用 Agent Base 上下文工具

```bash
# 在仓库根目录执行
python3 cli/agent-base doctor
python3 cli/agent-base context --format json
python3 cli/agent-base prompts show site-integration
python3 cli/agent-base runtime show debate-plugin
python3 cli/agent-base agent-service endpoints
python3 cli/agent-base agent-service smoke
python3 cli/agent-base mcp config --format json
```

### 示例请求

```bash
# 注册
curl -X POST http://localhost:8080/api/v1/users/register \
  -H "Content-Type: application/json" \
  -d '{"username":"demo","password":"123456","email":"demo@example.com"}'

# 登录
curl -X POST http://localhost:8080/api/v1/users/login \
  -H "Content-Type: application/json" \
  -d '{"username":"demo","password":"123456"}'
```

---

## 路线图

- [x] 提示词库初版（既有网站接入、新站点、ATH）
- [x] MCP Server 上下文工具
- [x] CLI 只读上下文工具
- [x] 通用外部 agent runtime 契约
- [x] agent-service 内存版 MVP
- [x] 可写 MCP proxy tools MVP
- [ ] 生产级 MCP 确认、幂等与审计 UI
- [ ] Agent Gateway（HTTPS webhook / Remote MCP）
- [ ] Session Orchestrator 与业务插件机制
- [ ] Agent Runtime 与任务管理
- [ ] 更多基础服务模板（文件服务、消息通知、权限管理）
- [ ] 多语言 SDK 支持

---

## 贡献指南

欢迎 Issue 和 Pull Request。提交前请确保：

1. 代码通过 `make lint` 检查
2. 新功能附有对应测试
3. 提交信息遵循 [Conventional Commits](https://www.conventionalcommits.org/) 规范

---

## License

[MIT](LICENSE)
