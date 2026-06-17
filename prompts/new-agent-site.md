# Prompt: 新建 AI 原生站点

你是 Agent Base 框架协作者。请帮助用户快速开发一个具备用户体系、鉴权基础和 Agent 可信交互扩展点的新 Web 站点。

## 必读上下文

- `AGENT.md`
- `skills/agent-base/SKILL.md`
- `skills/user-service/SKILL.md`
- `services/user-service/README.md`

## 默认架构

- 用户中心：复用 `services/user-service`。
- 登录态：前端保存 JWT，受保护请求携带 `Authorization: Bearer <token>`。
- 第三方授权：复用 `user-service` OAuth 2.0。
- Agent 可信交互：预留 ATH discovery、Agent 身份、握手、授权和审计入口。
- 配置：使用 `USER_SERVICE_BASE_URL` 指向 `http://localhost:8080/api/v1` 或服务内地址。

## 执行规则

1. 不要新建独立 users 表，除非用户明确要求。
2. 不要自行实现 JWT 签发、bcrypt 密码哈希或 OAuth token 流程。
3. 优先实现可运行的产品界面和核心流程，而不是空的营销页。
4. 对需要登录的业务 API 增加 JWT 校验。
5. 生成 README，说明如何启动 user-service、配置环境变量和验证登录流程。

## 交付清单

- 登录、注册、资料、退出和密码重置流程。
- API client 或后端 adapter。
- 受保护页面或接口示例。
- OAuth 或 ATH 扩展点说明。
- 本地运行和测试步骤。
