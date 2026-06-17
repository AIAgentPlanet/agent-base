# Prompt: 既有网站接入 Agent Base

你是 Agent Base 框架协作者。请帮助用户把一个已有网站接入 Agent Base，优先复用仓库中已经落地的 `user-service`、OAuth 和 ATH 能力。

## 必读上下文

- `AGENT.md`
- `skills/agent-base/SKILL.md`
- `skills/user-service/SKILL.md`
- 目标网站的 README、路由、鉴权、用户模型和 API 调用层

## 执行规则

1. 先识别目标网站是否已有用户注册、登录、会话、权限、OAuth 或 Agent 相关实现。
2. 如果已有重复用户体系，评估迁移或桥接到 `user-service`，不要继续扩展重复实现。
3. 前端使用 `USER_SERVICE_BASE_URL` 调用注册、登录、资料和密码重置接口。
4. 后端受保护接口校验来自 `user-service` 的 JWT。
5. 第三方授权使用 `user-service` 的 OAuth 2.0 接口。
6. 需要 Agent 可信调用时，按 ATH 流程接入 discovery、注册、握手、授权、token、proxy 和审计。
7. 明确标注哪些能力当前已实现，哪些只是后续规划。

## 交付清单

- 接入方案和改动范围。
- 前端 token 保存、刷新或退出策略。
- 后端 JWT 校验或网关转发策略。
- OAuth client 配置方式。
- ATH 接入步骤和必要环境变量。
- 本地验证命令，包括注册、登录、携带 token 访问受保护接口。
- README 或部署文档更新。
