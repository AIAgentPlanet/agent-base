---
name: agent-base
description: Use Agent Base prompts, skills, CLI, MCP, user-service, and ATH capabilities to plan or implement AI-native web application tasks. Triggers when working on Agent Base itself, adding agent capabilities to a site, or deciding how an agent should interact with the framework.
---

# Agent Base

当任务涉及 Agent Base 框架、AI 原生 Web 应用、站点接入、prompt/skill 编排、CLI/MCP 工具、user-service 或 ATH 可信交互时，先用本 skill 建立任务上下文。

## 能力边界

已落地能力：

- `services/user-service`：用户注册、登录、JWT、用户资料、密码重置、用户 CRUD、OAuth 2.0、ATH 可信交互、审计链和外部锚定。
- `skills/user-service/SKILL.md`：用户、OAuth、ATH 接入规则。
- `prompts/`：可复用任务提示词模板。
- `cli/agent-base`：本地上下文、prompt、skill、接口清单查询。
- `mcp/agent_base_mcp.py`：MCP stdio 服务器，暴露上下文工具和 agent-service proxy tools。
- `specs/agent-runtime/`：外部个人 agent 的通用连接、MCP tools、session schema 和 debate plugin 契约。
- `services/agent-service`：外部 agent runtime 内存版 MVP，提供 agent 注册、gateway connection、HTTP polling/WebSocket delivery/ack、session、participant、turn 和 message API。

未完整落地能力：

- 通用 Agent Runtime、长期任务调度、多 Agent 编排。
- 生产级 MCP 确认、幂等、审计 UI，以及创建用户、执行 ATH 握手等 user-service 写工具。
- 多服务模板，例如文件服务、通知服务、权限服务。
- 生产版 Agent Gateway（Webhook、可写 MCP adapter、持久化重投）、持久化 Session Orchestrator 和业务插件运行时。

## 决策流程

1. 判断任务是否需要用户、鉴权、OAuth 或 ATH。
2. 如果需要，必须阅读 `skills/user-service/SKILL.md`，优先复用 `user-service`。
3. 如果任务是改造已有网站，使用 `prompts/site-integration.md` 组织需求和交付项。
4. 如果任务是新建 AI 原生站点，使用 `prompts/new-agent-site.md` 组织技术方案。
5. 如果任务是治理、可信交互或 Agent 身份接入，使用 `prompts/ath-integration.md`。
6. 如果要给外部 Agent 暴露上下文或通用 session 操作，优先接入 `mcp/agent_base_mcp.py`。
7. 如果要在脚本或 CI 中检查框架状态，优先使用 `cli/agent-base doctor` 和 `cli/agent-base context`。
8. 如果任务涉及 Hermes、OpenClaw 或不同用户的外部个人 agent，先阅读 `specs/agent-runtime/README.md`、`connection-gateway.md`、`mcp-tools.md` 和相关 plugin 规格。
9. 新业务场景应设计为 session plugin，例如 `debate`，不要把业务语义写进 Agent Gateway。

## 交付标准

完成 Agent Base 相关任务时，尽量交付：

- 明确的能力边界：哪些是已实现调用，哪些是后续规划。
- 需要读取或复用的 prompt、skill、服务路径。
- user-service/OAuth/ATH 的接入路径、环境变量和验证步骤。
- CLI 或 MCP 调用示例。
- 对现有 Go/Sponge/Gin/GORM 分层结构的兼容说明。
- 通用层和业务插件的边界说明。

## 禁止事项

- 不要绕过 `user-service` 自建用户中心或 OAuth 服务。
- 不要把轻量 MCP 服务器描述成完整 Agent Runtime。
- 不要绕过 `agent-service` 直接在 MCP 工具里实现 session 或 gateway 状态变更。
- 不要在没有用户确认的情况下生成生产密钥、生产 OAuth client secret 或真实私钥。
