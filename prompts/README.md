# Agent Base Prompts

本目录存放给 AI Agent 复用的任务提示词模板。模板不是最终答案，而是用于稳定任务理解、约束实现边界和明确交付项。

## 模板

| 文件 | 场景 |
| --- | --- |
| `site-integration.md` | 改造已有网站，接入 Agent Base 的用户、鉴权、OAuth 或 ATH 能力 |
| `new-agent-site.md` | 从零开发具备用户体系和 Agent 可信交互基础的新站点 |
| `ath-integration.md` | 接入 ATH 可信握手、Agent 身份、审计链或外部锚定 |
| `external-agent-session.md` | Hermes、OpenClaw 等外部个人 agent 接入通用 session runtime |

## 使用方式

通过 CLI 查看模板：

```bash
python3 cli/agent-base prompts list
python3 cli/agent-base prompts show site-integration
```

通过 MCP 读取模板：

```json
{"name":"read_prompt","arguments":{"name":"site-integration"}}
```

## 编写约定

- 每个 prompt 都应说明适用场景、必要上下文、执行规则和交付清单。
- 涉及用户、登录、OAuth 或 ATH 时，必须要求读取 `skills/user-service/SKILL.md`。
- 涉及 Agent Base 框架能力时，必须要求读取 `skills/agent-base/SKILL.md`。
- 涉及外部个人 agent 或多 agent session 时，必须要求读取 `specs/agent-runtime/`。
- 不要在 prompt 中承诺尚未落地的 Runtime 能力。
