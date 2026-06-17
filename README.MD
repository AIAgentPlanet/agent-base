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
└── services/         # 基础微服务模板
    └── user-service  # 用户服务（注册/登录/鉴权）
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

### MCP Server & CLI

让 AI Agent 直接调用应用能力：

- 基于 [Model Context Protocol (MCP)](https://modelcontextprotocol.io) 标准实现
- 提供 CLI 工具，支持在终端和 CI/CD 流程中调用
- 低侵入接入，不依赖特定 AI 平台

当前提供轻量只读工具层：

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

# 启动 MCP stdio server
python3 mcp/agent_base_mcp.py
```

MCP 当前暴露 `agent_base_context`、`list_prompts`、`read_prompt`、`list_skills`、`read_skill`、`user_service_endpoints` 和 `ath_endpoints` 等只读工具。完整 Agent Runtime、长期任务调度和可写 MCP 工具仍属于后续扩展。

### 基础代码库 (Services)

经过验证的微服务模板，AI 基于此生成代码时风格一致、质量可控：

| 模块 | 技术栈 | 说明 |
|------|--------|------|
| user-service | Go · Gin · PostgreSQL · Redis · JWT · OAuth2 · ATH | 用户注册、登录、信息管理、OAuth、Agent 可信交互 |

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

### 使用 Agent Base 上下文工具

```bash
# 在仓库根目录执行
python3 cli/agent-base doctor
python3 cli/agent-base context --format json
python3 cli/agent-base prompts show site-integration
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
- [x] MCP Server 只读上下文工具
- [x] CLI 只读上下文工具
- [ ] 可写 MCP 工具与认证审计
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
