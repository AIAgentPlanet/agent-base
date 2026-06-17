# Agent Base MCP Server

`mcp/agent_base_mcp.py` 是一个轻量 MCP stdio 服务器，用于让外部 AI Agent 读取 Agent Base 的只读上下文。

## 启动

```bash
python3 mcp/agent_base_mcp.py
```

CLI 可输出客户端配置：

```bash
python3 cli/agent-base mcp config --format json
```

## 工具

| 工具 | 说明 |
| --- | --- |
| `agent_base_context` | 返回 Agent Base 已实现能力、边界和入口文件 |
| `list_prompts` | 列出 prompt 模板 |
| `read_prompt` | 读取指定 prompt，参数：`name` |
| `list_skills` | 列出 skill |
| `read_skill` | 读取指定 skill，参数：`name` |
| `user_service_endpoints` | 返回用户与 OAuth 接口清单 |
| `ath_endpoints` | 返回 ATH 接口清单 |

## 设计边界

- 当前服务器只暴露只读工具。
- 不直接调用数据库或写入 `user-service`。
- 不生成私钥、client secret 或生产配置。
- 后续需要增加可写工具时，必须先补认证、审计、确认和幂等策略。
