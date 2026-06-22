#!/usr/bin/env python3
import json
import os
import sys
from pathlib import Path
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


ROOT = Path(__file__).resolve().parents[1]
PROMPTS_DIR = ROOT / "prompts"
SKILLS_DIR = ROOT / "skills"
SPECS_DIR = ROOT / "specs" / "agent-runtime"
AGENT_SERVICE_URL = os.environ.get("AGENT_BASE_AGENT_SERVICE_URL", "http://localhost:8090").rstrip("/")


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
    {"method": "POST", "path": "/api/v1/ath/introspect", "description": "ATH token introspection / active 状态查询", "auth": "client_secret"},
    {"method": "POST", "path": "/api/v1/ath/proxy", "description": "ATH API 代理", "auth": "ath_bearer"},
    {"method": "POST", "path": "/api/v1/ath/audit/query", "description": "审计记录查询", "auth": "client_secret"},
    {"method": "POST", "path": "/api/v1/ath/audit/verify", "description": "审计链校验", "auth": "client_secret"},
    {"method": "POST", "path": "/api/v1/ath/audit/anchor/status", "description": "锚定状态", "auth": "client_secret"},
    {"method": "POST", "path": "/api/v1/ath/audit/anchor/retry", "description": "锚定重试", "auth": "client_secret"},
]

AGENT_SERVICE_ENDPOINTS = [
    {"method": "GET", "path": "/healthz", "description": "健康检查", "auth": "public"},
    {"method": "GET", "path": "/api/v1/runtime/discover", "description": "发现 agent-service MVP 能力", "auth": "public"},
    {"method": "POST", "path": "/api/v1/agents", "description": "注册外部 agent 连接档案", "auth": "mvp_no_auth"},
    {"method": "GET", "path": "/api/v1/agents/:id", "description": "查询 agent", "auth": "mvp_no_auth"},
    {"method": "POST", "path": "/api/v1/connections", "description": "注册 agent gateway connection，返回一次性 connection_token", "auth": "mvp_no_auth"},
    {"method": "GET", "path": "/api/v1/connections/:id/deliveries/next", "description": "拉取下一条 gateway delivery", "auth": "agent_connection_token"},
    {"method": "POST", "path": "/api/v1/connections/:id/deliveries/:deliveryId/ack", "description": "确认 gateway delivery 已收到", "auth": "agent_connection_token"},
    {"method": "GET", "path": "/api/v1/connections/:id/ws", "description": "WebSocket gateway：接收 delivery 并发送 delivery.ack", "auth": "agent_connection_token"},
    {"method": "POST", "path": "/api/v1/sessions", "description": "创建通用 session", "auth": "mvp_no_auth"},
    {"method": "GET", "path": "/api/v1/sessions/:id", "description": "查询 session", "auth": "mvp_no_auth"},
    {"method": "POST", "path": "/api/v1/sessions/:id/participants", "description": "加入 session", "auth": "mvp_no_auth"},
    {"method": "GET", "path": "/api/v1/sessions/:id/participants/:participantId/next-turn", "description": "获取下一轮 turn", "auth": "participant_session_token"},
    {"method": "POST", "path": "/api/v1/sessions/:id/messages", "description": "提交 session message", "auth": "participant_session_token"},
    {"method": "GET", "path": "/api/v1/sessions/:id/messages", "description": "查询 session messages", "auth": "mvp_no_auth"},
    {"method": "GET", "path": "/api/v1/sessions/:id/audit", "description": "查询审计状态；配置 ATH 后代理到 user-service", "auth": "mvp_no_auth"},
]

CONNECTION_MODES = [
    {"name": "https_webhook", "description": "Agent Base calls a public HTTPS endpoint owned by the agent."},
    {"name": "websocket", "description": "Agent connects outward and receives turn deliveries over a long-lived channel."},
    {"name": "mcp", "description": "Agent uses Agent Base MCP tools to discover sessions, fetch turns, and submit results."},
]

SESSION_TYPES = [
    {
        "name": "debate",
        "description": "Two or more user-authorized agents exchange arguments under a turn policy.",
        "spec": "debate-plugin",
    },
]

RUNTIME_TOOLS = [
    {"name": "agent_base.discover", "mode": "read", "description": "Return runtime capabilities, connection modes, and session types."},
    {"name": "agent_base.list_session_types", "mode": "read", "description": "List supported session types."},
    {"name": "agent_base.get_session_schema", "mode": "read", "description": "Return schema for a session type."},
    {"name": "agent_base.get_audit_status", "mode": "read", "description": "Return ATH audit status for a session."},
    {"name": "agent_base.register_connection", "mode": "write", "description": "Register an agent gateway connection and return a connection token."},
    {"name": "agent_base.next_delivery", "mode": "write", "description": "Fetch the next gateway delivery for a connection."},
    {"name": "agent_base.register_agent", "mode": "write", "description": "Register an external agent identity and connection profile."},
    {"name": "agent_base.bind_agent_to_user", "mode": "write-planned", "description": "Bind a verified agent to a user."},
    {"name": "agent_base.create_session", "mode": "write", "description": "Create a generic runtime session."},
    {"name": "agent_base.join_session", "mode": "write", "description": "Join a session as an authorized participant."},
    {"name": "agent_base.get_session", "mode": "read", "description": "Fetch a session by id."},
    {"name": "agent_base.get_next_turn", "mode": "write", "description": "Fetch the next turn assigned to the current agent."},
    {"name": "agent_base.submit_message", "mode": "write", "description": "Submit a session message."},
    {"name": "agent_base.submit_artifact", "mode": "write-planned", "description": "Submit a structured artifact."},
    {"name": "agent_base.ack_delivery", "mode": "write", "description": "Acknowledge an async delivery."},
    {"name": "agent_base.revoke_session_token", "mode": "write-planned", "description": "Revoke current session authorization."},
]


def text_resource(text):
    return {"content": [{"type": "text", "text": text}]}


def json_resource(data):
    return text_resource(json.dumps(data, ensure_ascii=False, indent=2))


def agent_service_request(method, path, body=None, token=None, token_header=None):
    headers = {"Accept": "application/json"}
    data = None
    if body is not None:
        data = json.dumps(body, ensure_ascii=False).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if token:
        headers[token_header or "Authorization"] = token if token_header else f"Bearer {token}"
    request = Request(AGENT_SERVICE_URL + path, data=data, headers=headers, method=method)
    try:
        with urlopen(request, timeout=10) as response:
            payload = response.read().decode("utf-8")
            return json.loads(payload) if payload else None
    except HTTPError as exc:
        payload = exc.read().decode("utf-8")
        try:
            detail = json.loads(payload)
        except json.JSONDecodeError:
            detail = payload
        raise RuntimeError(f"agent-service {exc.code}: {detail}") from exc
    except URLError as exc:
        raise RuntimeError(f"agent-service unavailable at {AGENT_SERVICE_URL}: {exc.reason}") from exc


def pick(arguments, *names):
    return {name: arguments[name] for name in names if name in arguments}


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


def read_runtime_spec(name):
    if name.endswith(".json"):
        path = SPECS_DIR / name
    else:
        path = SPECS_DIR / f"{name}.md"
    if not path.exists():
        raise ValueError(f"Unknown runtime spec: {name}")
    return path.read_text(encoding="utf-8")


def agent_base_context():
    return {
        "name": "agent-base",
        "root": str(ROOT),
        "implemented": [
            "user-service: users, JWT, OAuth 2.0, ATH",
            "agent-service: in-memory external-agent runtime MVP",
            "prompts: reusable task templates",
            "skills: agent-base and user-service guidance",
            "cli: local context and endpoint catalog tool",
            "mcp: stdio context server with agent-service proxy tools",
            "agent-runtime specs: generic gateway, MCP tools, session schema, debate plugin",
        ],
        "not_full_runtime": [
            "production Agent Runtime service",
            "long-running task scheduler",
            "production-grade MCP auth confirmation and persistence",
            "persistent multi-agent orchestration",
        ],
        "entrypoints": {
            "agent_instructions": "AGENT.md",
            "agent_base_skill": "skills/agent-base/SKILL.md",
            "user_service_skill": "skills/user-service/SKILL.md",
            "prompts": "prompts/",
            "cli": "cli/agent-base",
            "mcp": "mcp/agent_base_mcp.py",
            "runtime_specs": "specs/agent-runtime",
            "agent_service": "services/agent-service",
            "user_service": "services/user-service",
        },
        "connection_modes": CONNECTION_MODES,
        "session_types": SESSION_TYPES,
    }


def agent_runtime_discover():
    return {
        "name": "agent-base-runtime",
        "version": "0.1.0",
        "security": {
            "identity": "ATH",
            "authorization": "per-session scoped token",
            "audit": "ATH audit chain",
        },
        "connection_modes": CONNECTION_MODES,
        "session_types": SESSION_TYPES,
        "tools": RUNTIME_TOOLS,
        "specs": sorted(path.name for path in SPECS_DIR.iterdir() if path.is_file()),
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
    {
        "name": "agent_service_endpoints",
        "description": "Return agent-service MVP endpoint catalog.",
        "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False},
    },
    {
        "name": "agent_runtime_discover",
        "description": "Return generic external-agent runtime capabilities and contracts.",
        "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False},
    },
    {
        "name": "list_connection_modes",
        "description": "List supported external-agent connection modes.",
        "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False},
    },
    {
        "name": "list_session_types",
        "description": "List supported generic session types.",
        "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False},
    },
    {
        "name": "list_runtime_tools",
        "description": "List generic runtime MCP tool contracts.",
        "inputSchema": {"type": "object", "properties": {}, "additionalProperties": False},
    },
    {
        "name": "read_runtime_spec",
        "description": "Read a runtime contract spec by name.",
        "inputSchema": {
            "type": "object",
            "properties": {"name": {"type": "string"}},
            "required": ["name"],
            "additionalProperties": False,
        },
    },
    {
        "name": "agent_base_register_agent",
        "description": "Create an external agent profile through agent-service.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "user_id": {"type": "string"},
                "type": {"type": "string"},
                "identity": {"type": "string"},
                "display_name": {"type": "string"},
                "connection_mode": {"type": "string"},
                "capabilities": {"type": "array", "items": {"type": "string"}},
                "metadata": {"type": "object", "additionalProperties": {"type": "string"}},
            },
            "required": ["user_id", "identity"],
            "additionalProperties": False,
        },
    },
    {
        "name": "agent_base_create_session",
        "description": "Create a generic session through agent-service.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "type": {"type": "string"},
                "owner_user_id": {"type": "string"},
                "policy": {"type": "object"},
                "metadata": {"type": "object", "additionalProperties": {"type": "string"}},
            },
            "required": ["owner_user_id"],
            "additionalProperties": False,
        },
    },
    {
        "name": "agent_base_join_session",
        "description": "Join a session as an agent participant. Returns a one-time session_token.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "session_id": {"type": "string"},
                "user_id": {"type": "string"},
                "agent_id": {"type": "string"},
                "role": {"type": "string"},
                "scopes": {"type": "array", "items": {"type": "string"}},
            },
            "required": ["session_id", "user_id", "agent_id"],
            "additionalProperties": False,
        },
    },
    {
        "name": "agent_base_get_session",
        "description": "Fetch a session by id.",
        "inputSchema": {
            "type": "object",
            "properties": {"session_id": {"type": "string"}},
            "required": ["session_id"],
            "additionalProperties": False,
        },
    },
    {
        "name": "agent_base_register_connection",
        "description": "Create a gateway connection. Returns a one-time connection_token.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "agent_id": {"type": "string"},
                "mode": {"type": "string"},
                "metadata": {"type": "object", "additionalProperties": {"type": "string"}},
            },
            "required": ["agent_id"],
            "additionalProperties": False,
        },
    },
    {
        "name": "agent_base_next_delivery",
        "description": "Fetch the next gateway delivery for a connection.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "connection_id": {"type": "string"},
                "connection_token": {"type": "string"},
            },
            "required": ["connection_id", "connection_token"],
            "additionalProperties": False,
        },
    },
    {
        "name": "agent_base_ack_delivery",
        "description": "Acknowledge a gateway delivery for a connection.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "connection_id": {"type": "string"},
                "delivery_id": {"type": "string"},
                "connection_token": {"type": "string"},
                "status": {"type": "string"},
            },
            "required": ["connection_id", "delivery_id", "connection_token"],
            "additionalProperties": False,
        },
    },
    {
        "name": "agent_base_get_next_turn",
        "description": "Fetch the next turn for a session participant.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "session_id": {"type": "string"},
                "participant_id": {"type": "string"},
                "session_token": {"type": "string"},
            },
            "required": ["session_id", "participant_id", "session_token"],
            "additionalProperties": False,
        },
    },
    {
        "name": "agent_base_submit_message",
        "description": "Submit a session message for a participant.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "session_id": {"type": "string"},
                "participant_id": {"type": "string"},
                "turn_id": {"type": "string"},
                "type": {"type": "string"},
                "content": {"type": "string"},
                "audit_ref": {"type": "string"},
                "session_token": {"type": "string"},
            },
            "required": ["session_id", "participant_id", "turn_id", "content", "session_token"],
            "additionalProperties": False,
        },
    },
    {
        "name": "agent_base_list_messages",
        "description": "List messages for a session.",
        "inputSchema": {
            "type": "object",
            "properties": {"session_id": {"type": "string"}},
            "required": ["session_id"],
            "additionalProperties": False,
        },
    },
    {
        "name": "agent_base_get_audit_status",
        "description": "Get audit status for a session.",
        "inputSchema": {
            "type": "object",
            "properties": {"session_id": {"type": "string"}},
            "required": ["session_id"],
            "additionalProperties": False,
        },
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
    if name == "agent_service_endpoints":
        return json_resource(AGENT_SERVICE_ENDPOINTS)
    if name == "agent_runtime_discover":
        return json_resource(agent_runtime_discover())
    if name == "list_connection_modes":
        return json_resource(CONNECTION_MODES)
    if name == "list_session_types":
        return json_resource(SESSION_TYPES)
    if name == "list_runtime_tools":
        return json_resource(RUNTIME_TOOLS)
    if name == "read_runtime_spec":
        return text_resource(read_runtime_spec(arguments["name"]))
    if name == "agent_base_register_agent":
        body = pick(arguments, "user_id", "type", "identity", "display_name", "connection_mode", "capabilities", "metadata")
        return json_resource(agent_service_request("POST", "/api/v1/agents", body))
    if name == "agent_base_create_session":
        body = pick(arguments, "type", "owner_user_id", "policy", "metadata")
        return json_resource(agent_service_request("POST", "/api/v1/sessions", body))
    if name == "agent_base_join_session":
        session_id = arguments["session_id"]
        body = pick(arguments, "user_id", "agent_id", "role", "scopes")
        return json_resource(agent_service_request("POST", f"/api/v1/sessions/{session_id}/participants", body))
    if name == "agent_base_get_session":
        return json_resource(agent_service_request("GET", f"/api/v1/sessions/{arguments['session_id']}"))
    if name == "agent_base_register_connection":
        body = pick(arguments, "agent_id", "mode", "metadata")
        return json_resource(agent_service_request("POST", "/api/v1/connections", body))
    if name == "agent_base_next_delivery":
        path = f"/api/v1/connections/{arguments['connection_id']}/deliveries/next"
        return json_resource(agent_service_request("GET", path, token=arguments["connection_token"], token_header="X-Agent-Connection-Token"))
    if name == "agent_base_ack_delivery":
        path = f"/api/v1/connections/{arguments['connection_id']}/deliveries/{arguments['delivery_id']}/ack"
        body = {"status": arguments.get("status", "ok")}
        return json_resource(agent_service_request("POST", path, body, token=arguments["connection_token"], token_header="X-Agent-Connection-Token"))
    if name == "agent_base_get_next_turn":
        path = f"/api/v1/sessions/{arguments['session_id']}/participants/{arguments['participant_id']}/next-turn"
        return json_resource(agent_service_request("GET", path, token=arguments["session_token"]))
    if name == "agent_base_submit_message":
        path = f"/api/v1/sessions/{arguments['session_id']}/messages"
        body = pick(arguments, "participant_id", "turn_id", "type", "content", "audit_ref")
        if "type" not in body:
            body["type"] = "message"
        return json_resource(agent_service_request("POST", path, body, token=arguments["session_token"]))
    if name == "agent_base_list_messages":
        return json_resource(agent_service_request("GET", f"/api/v1/sessions/{arguments['session_id']}/messages"))
    if name == "agent_base_get_audit_status":
        return json_resource(agent_service_request("GET", f"/api/v1/sessions/{arguments['session_id']}/audit"))
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
