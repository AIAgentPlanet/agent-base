# Agent Base CLI

`cli/agent-base` 是一个轻量、零第三方依赖的本地工具，用于让开发者、CI 和 AI Agent 快速读取 Agent Base 的上下文。

## 用法

```bash
python3 cli/agent-base doctor
python3 cli/agent-base context --format markdown
python3 cli/agent-base prompts list
python3 cli/agent-base prompts show site-integration
python3 cli/agent-base skills list
python3 cli/agent-base skills show agent-base
python3 cli/agent-base user-service endpoints
python3 cli/agent-base runtime connection-modes
python3 cli/agent-base runtime session-types
python3 cli/agent-base runtime tools
python3 cli/agent-base runtime show connection-gateway
python3 cli/agent-base mcp config
```

## 设计边界

- CLI 当前只做只读查询和上下文生成。
- CLI 不创建用户、不写数据库、不生成生产密钥。
- 需要真实调用服务时，使用 README 中的 curl 示例、`agent-service` HTTP API，或 `mcp/agent_base_mcp.py` 的 agent-service proxy tools。
- `runtime` 子命令暴露的是通用外部 agent 接入契约，不代表生产版 Gateway 或 Orchestrator 已经落地。
