#!/usr/bin/env python3
import json
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]
PROMPTS_DIR = ROOT / "prompts"
SKILLS_DIR = ROOT / "skills"


USER_SERVICE_ENDPOINTS = [
    {"method": "POST", "path": "/users/register", "description": "用户注册", "auth": "public"},
    {"method": "POST", "path": "/users/login", "description": "用户登录，返回 JWT", "auth": "public"},
    {"method": "GET", "path": "/users/profile", "description": "当前用户资料", "auth": "jwt"},
    {"method": "PUT", "path": "/users/profile", "description": "更新当前用户资料", "auth": "jwt"},
    {"method": "POST", "path": "/users/reset-code", "description": "发送密码重置验证码", "auth": "public"},
    {"method": "POST", "path": "/users/reset-password", "description": "重置密码", "auth": "public"},
    {"method": "POST", "path": "/oauth/clients", "description": "创建 OAuth Client", "auth": "jwt"},
    {"method": "POST", "path": "/oauth/clients/list", "description": "列出 OAuth Client", "auth": "jwt"},
    {"method": "GET", "path": "/oauth/authorize", "description": "OAuth 授权端点", "auth": "jwt"},
    {"method": "POST", "path": "/oauth/token", "description": "OAuth token 端点", "auth": "public"},
    {"method": "GET", "path": "/oauth/userinfo", "description": "OAuth 用户信息", "auth": "oauth_bearer"},
    {"method": "POST", "path": "/oauth/revoke", "description": "OAuth token 吊销", "auth": "public"},
]

ATH_ENDPOINTS = [
    {"method": "GET", "path": "/.well-known/ath.json", "description": "ATH 发现文档", "auth": "public"},
    {"method": "GET", "path": "/.well-known/did.json", "description": "服务端 DID 文档", "auth": "public"},
    {"method": "GET", "path": "/.well-known/ath-audit-head.json", "description": "公开审计链头", "auth": "public"},
    {"method": "POST", "path": "/api/v1/ath/agents/register", "description": "Agent 注册", "auth": "attestation_jwt"},
    {"method": "GET", "path": "/api/v1/ath/agents/:clientId", "description": "Agent 状态", "auth": "attestation_jwt"},
    {"method": "POST", "path": "/api/v1/ath/handshakes", "description": "发起身份握手", "auth": "registered_agent"},
    {"method": "POST", "path": "/api/v1/ath/handshakes/:handshakeId/proof", "description": "提交身份签名", "auth": "es256_signature"},
    {"method": "GET", "path": "/api/v1/ath/handshakes/:handshakeId", "description": "握手状态", "auth": "client_id"},
    {"method": "POST", "path": "/api/v1/ath/authorize", "description": "用户授权绑定", "auth": "attestation_jwt"},
    {"method": "POST", "path": "/api/v1/ath/token", "description": "ATH token 交换", "auth": "client_secret"},
    {"method": "POST", "path": "/api/v1/ath/revoke", "description": "ATH token 注销", "auth": "client_secret"},
    {"method": "POST", "path": "/api/v1/ath/proxy", "description": "ATH API 代理", "auth": "ath_bearer"},
    {"method": "POST", "path": "/api/v1/ath/audit/query", "description": "审计记录查询", "auth": "client_secret"},
    {"method": "POST", "path": "/api/v1/ath/audit/verify", "description": "审计链校验", "auth": "client_secret"},
    {"method": "POST", "path": "/api/v1/ath/audit/anchor/status", "description": "锚定状态", "auth": "client_secret"},
    {"method": "POST", "path": "/api/v1/ath/audit/anchor/retry", "description": "锚定重试", "auth": "client_secret"},
]


def text_resource(text):
    return {"content": [{"type": "text", "text": text}]}


def json_resource(data):
    return text_resource(json.dumps(data, ensure_ascii=False, indent=2))


def list_prompts():
    return sorted(path.stem for path in PROMPTS_DIR.glob("*.md") if path.name != "README.md")


def list_skills():
    return sorted(path.parent.name for path in SKILLS_DIR.glob("*/SKILL.md"))


def read_prompt(name):
    path = PROMPTS_DIR / f"{name}.md"
    if not path.exists():
        raise ValueError(f"Unknown prompt: {name}")
    return path.read_text(encoding="utf-8")


def read_skill(name):
    path = SKILLS_DIR / name / "SKILL.md"
    if not path.exists():
        raise ValueError(f"Unknown skill: {name}")
    return path.read_text(encoding="utf-8")


def agent_base_context():
    return {
        "name": "agent-base",
        "root": str(ROOT),
        "implemented": [
            "user-service: users, JWT, OAuth 2.0, ATH",
            "prompts: reusable task templates",
            "skills: agent-base and user-service guidance",
            "cli: read-only local context tool",
            "mcp: read-only stdio context server",
        ],
        "not_full_runtime": [
            "general Agent Runtime",
            "long-running task scheduler",
            "state-changing MCP tools",
            "multi-agent orchestration",
        ],
        "entrypoints": {
            "agent_instructions": "AGENT.md",
            "agent_base_skill": "skills/agent-base/SKILL.md",
            "user_service_skill": "skills/user-service/SKILL.md",
            "prompts": "prompts/",
            "cli": "cli/agent-base",
            "mcp": "mcp/agent_base_mcp.py",
            "user_service": "services/user-service",
        },
    }


TOOLS = [
    {
        "name": "agent_base_context",
        "description": "Return implemented Agent Base capabilities, boundaries, and entrypoints.",
        "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False},
    },
    {
        "name": "list_prompts",
        "description": "List reusable Agent Base prompt templates.",
        "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False},
    },
    {
        "name": "read_prompt",
        "description": "Read a prompt template by name.",
        "inputSchema": {
            "type": "object",
            "properties": {"name": {"type": "string"}},
            "required": ["name"],
            "additionalProperties": False,
        },
    },
    {
        "name": "list_skills",
        "description": "List Agent Base skills.",
        "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False},
    },
    {
        "name": "read_skill",
        "description": "Read a skill by name.",
        "inputSchema": {
            "type": "object",
            "properties": {"name": {"type": "string"}},
            "required": ["name"],
            "additionalProperties": False,
        },
    },
    {
        "name": "user_service_endpoints",
        "description": "Return user-service user and OAuth endpoint catalog.",
        "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False},
    },
    {
        "name": "ath_endpoints",
        "description": "Return ATH endpoint catalog.",
        "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False},
    },
]


def call_tool(name, arguments):
    arguments = arguments or {}
    if name == "agent_base_context":
        return json_resource(agent_base_context())
    if name == "list_prompts":
        return json_resource(list_prompts())
    if name == "read_prompt":
        return text_resource(read_prompt(arguments["name"]))
    if name == "list_skills":
        return json_resource(list_skills())
    if name == "read_skill":
        return text_resource(read_skill(arguments["name"]))
    if name == "user_service_endpoints":
        return json_resource(USER_SERVICE_ENDPOINTS)
    if name == "ath_endpoints":
        return json_resource(ATH_ENDPOINTS)
    raise ValueError(f"Unknown tool: {name}")


def respond(message_id, result=None, error=None):
    payload = {"jsonrpc": "2.0", "id": message_id}
    if error is not None:
        payload["error"] = error
    else:
        payload["result"] = result
    sys.stdout.write(json.dumps(payload, ensure_ascii=False) + "\n")
    sys.stdout.flush()


def handle(request):
    method = request.get("method")
    params = request.get("params") or {}
    message_id = request.get("id")

    if method == "initialize":
        return {
            "protocolVersion": params.get("protocolVersion", "2024-11-05"),
            "capabilities": {"tools": {}},
            "serverInfo": {"name": "agent-base-mcp", "version": "0.1.0"},
        }
    if method == "tools/list":
        return {"tools": TOOLS}
    if method == "tools/call":
        return call_tool(params.get("name"), params.get("arguments") or {})
    if method == "notifications/initialized":
        return None
    if message_id is None:
        return None
    raise ValueError(f"Unsupported method: {method}")


def main():
    for line in sys.stdin:
        if not line.strip():
            continue
        try:
            request = json.loads(line)
            result = handle(request)
            if request.get("id") is not None:
                respond(request.get("id"), result=result)
        except Exception as exc:
            message_id = None
            try:
                message_id = json.loads(line).get("id")
            except Exception:
                pass
            respond(message_id, error={"code": -32000, "message": str(exc)})


if __name__ == "__main__":
    main()
