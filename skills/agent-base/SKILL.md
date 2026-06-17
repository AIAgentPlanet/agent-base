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
- `mcp/agent_base_mcp.py`：MCP stdio 服务器，暴露只读上下文工具。

未完整落地能力：

- 通用 Agent Runtime、长期任务调度、多 Agent 编排。
- 可写型 MCP 工具，例如创建用户、注册 Agent、执行 ATH 握手。
- 多服务模板，例如文件服务、通知服务、权限服务。

## 决策流程

1. 判断任务是否需要用户、鉴权、OAuth 或 ATH。
2. 如果需要，必须阅读 `skills/user-service/SKILL.md`，优先复用 `user-service`。
3. 如果任务是改造已有网站，使用 `prompts/site-integration.md` 组织需求和交付项。
4. 如果任务是新建 AI 原生站点，使用 `prompts/new-agent-site.md` 组织技术方案。
5. 如果任务是治理、可信交互或 Agent 身份接入，使用 `prompts/ath-integration.md`。
6. 如果要给外部 Agent 暴露上下文，优先接入 `mcp/agent_base_mcp.py` 的只读工具。
7. 如果要在脚本或 CI 中检查框架状态，优先使用 `cli/agent-base doctor` 和 `cli/agent-base context`。

## 交付标准

完成 Agent Base 相关任务时，尽量交付：

- 明确的能力边界：哪些是已实现调用，哪些是后续规划。
- 需要读取或复用的 prompt、skill、服务路径。
- user-service/OAuth/ATH 的接入路径、环境变量和验证步骤。
- CLI 或 MCP 调用示例。
- 对现有 Go/Sponge/Gin/GORM 分层结构的兼容说明。

## 禁止事项

- 不要绕过 `user-service` 自建用户中心或 OAuth 服务。
- 不要把轻量 MCP 服务器描述成完整 Agent Runtime。
- 不要让只读 MCP 工具执行会修改状态的操作。
- 不要在没有用户确认的情况下生成生产密钥、生产 OAuth client secret 或真实私钥。
