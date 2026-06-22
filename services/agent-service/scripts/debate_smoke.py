#!/usr/bin/env python3
import argparse
import json
import os
import sys
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


DEFAULT_BASE_URL = os.environ.get("AGENT_BASE_AGENT_SERVICE_URL", "http://localhost:8090").rstrip("/")


def request_json(method, base_url, path, body=None, token=None, token_header=None):
    headers = {"Accept": "application/json"}
    data = None
    if body is not None:
        data = json.dumps(body, ensure_ascii=False).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if token:
        if token_header:
            headers[token_header] = token
        else:
            headers["Authorization"] = f"Bearer {token}"
    request = Request(base_url + path, data=data, headers=headers, method=method)
    try:
        with urlopen(request, timeout=10) as response:
            payload = response.read().decode("utf-8")
            return json.loads(payload) if payload else None
    except HTTPError as exc:
        payload = exc.read().decode("utf-8")
        raise SystemExit(f"{method} {path} failed with HTTP {exc.code}: {payload}") from exc
    except URLError as exc:
        raise SystemExit(f"agent-service unavailable at {base_url}: {exc.reason}") from exc


def assert_equal(actual, expected, message):
    if actual != expected:
        raise SystemExit(f"{message}: expected {expected!r}, got {actual!r}")


def run_smoke(base_url, topic):
    health = request_json("GET", base_url, "/healthz")
    assert_equal(health.get("status"), "ok", "health check failed")

    agent_a = request_json(
        "POST",
        base_url,
        "/api/v1/agents",
        {
            "user_id": "smoke_user_a",
            "type": "hermes",
            "identity": "https://smoke-a.example/.well-known/agent.json",
            "display_name": "Smoke Hermes",
            "connection_mode": "mcp",
            "capabilities": ["session.read", "session.speak"],
        },
    )
    agent_b = request_json(
        "POST",
        base_url,
        "/api/v1/agents",
        {
            "user_id": "smoke_user_b",
            "type": "openclaw",
            "identity": "https://smoke-b.example/.well-known/agent.json",
            "display_name": "Smoke OpenClaw",
            "connection_mode": "mcp",
            "capabilities": ["session.read", "session.speak"],
        },
    )
    session = request_json(
        "POST",
        base_url,
        "/api/v1/sessions",
        {
            "type": "debate",
            "owner_user_id": "smoke_user_a",
            "policy": {
                "turn_policy": "alternate",
                "max_turns": 2,
                "allowed_message_types": ["argument"],
                "audit_required": True,
            },
            "metadata": {"topic": topic},
        },
    )
    participant_a = request_json(
        "POST",
        base_url,
        f"/api/v1/sessions/{session['id']}/participants",
        {
            "user_id": "smoke_user_a",
            "agent_id": agent_a["id"],
            "role": "affirmative",
            "scopes": ["session:read", "session:speak"],
        },
    )
    participant_b = request_json(
        "POST",
        base_url,
        f"/api/v1/sessions/{session['id']}/participants",
        {
            "user_id": "smoke_user_b",
            "agent_id": agent_b["id"],
            "role": "negative",
            "scopes": ["session:read", "session:speak"],
        },
    )
    connection_a = request_json("POST", base_url, "/api/v1/connections", {"agent_id": agent_a["id"], "mode": "mcp"})
    connection_b = request_json("POST", base_url, "/api/v1/connections", {"agent_id": agent_b["id"], "mode": "mcp"})

    run_turn(
        base_url,
        connection_a,
        participant_a,
        "argument",
        "正方：Agent Base 的 MCP gateway 能把个人 agent 纳入同一场 session。",
    )
    run_turn(
        base_url,
        connection_b,
        participant_b,
        "argument",
        "反方：MVP 已经可验证闭环，但生产还需要确认、幂等和审计体验。",
    )

    messages = request_json("GET", base_url, f"/api/v1/sessions/{session['id']}/messages")
    final_session = request_json("GET", base_url, f"/api/v1/sessions/{session['id']}")
    audit = request_json("GET", base_url, f"/api/v1/sessions/{session['id']}/audit")

    assert_equal(len(messages), 2, "message count mismatch")
    assert_equal(final_session["status"], "completed", "session status mismatch")

    return {
        "status": "ok",
        "base_url": base_url,
        "session_id": session["id"],
        "session_status": final_session["status"],
        "agent_ids": [agent_a["id"], agent_b["id"]],
        "participant_ids": [participant_a["id"], participant_b["id"]],
        "connection_ids": [connection_a["id"], connection_b["id"]],
        "message_ids": [message["id"] for message in messages],
        "audit_status": audit.get("status"),
    }


def run_turn(base_url, connection, participant, message_type, content):
    delivery = request_json(
        "GET",
        base_url,
        f"/api/v1/connections/{connection['id']}/deliveries/next",
        token=connection["connection_token"],
        token_header="X-Agent-Connection-Token",
    )
    assert_equal(delivery["type"], "turn.available", "delivery type mismatch")
    assert_equal(delivery["participant_id"], participant["id"], "delivery participant mismatch")
    request_json(
        "POST",
        base_url,
        f"/api/v1/connections/{connection['id']}/deliveries/{delivery['id']}/ack",
        {"status": "ok"},
        token=connection["connection_token"],
        token_header="X-Agent-Connection-Token",
    )
    turn = request_json(
        "GET",
        base_url,
        f"/api/v1/sessions/{participant['session_id']}/participants/{participant['id']}/next-turn",
        token=participant["session_token"],
    )
    assert_equal(turn["id"], delivery["turn_id"], "turn/delivery mismatch")
    return request_json(
        "POST",
        base_url,
        f"/api/v1/sessions/{participant['session_id']}/messages",
        {
            "participant_id": participant["id"],
            "turn_id": turn["id"],
            "type": message_type,
            "content": content,
            "audit_ref": "smoke",
        },
        token=participant["session_token"],
    )


def dry_run_plan(base_url, topic):
    return {
        "base_url": base_url,
        "topic": topic,
        "steps": [
            "GET /healthz",
            "POST /api/v1/agents x2",
            "POST /api/v1/sessions",
            "POST /api/v1/sessions/:id/participants x2",
            "POST /api/v1/connections x2",
            "GET /api/v1/connections/:id/deliveries/next",
            "POST /api/v1/connections/:id/deliveries/:deliveryId/ack",
            "GET /api/v1/sessions/:id/participants/:participantId/next-turn",
            "POST /api/v1/sessions/:id/messages",
            "repeat delivery/ack/turn/message for second participant",
            "GET /api/v1/sessions/:id/messages",
            "GET /api/v1/sessions/:id",
            "GET /api/v1/sessions/:id/audit",
        ],
    }


def main(argv=None):
    parser = argparse.ArgumentParser(description="Run a two-agent debate smoke against agent-service.")
    parser.add_argument("--base-url", default=DEFAULT_BASE_URL, help="agent-service base URL")
    parser.add_argument("--topic", default="MCP adapter 是否足以承载个人 agent 辩论", help="debate topic")
    parser.add_argument("--dry-run", action="store_true", help="print the smoke plan without calling the service")
    args = parser.parse_args(argv)

    if args.dry_run:
        result = dry_run_plan(args.base_url.rstrip("/"), args.topic)
    else:
        result = run_smoke(args.base_url.rstrip("/"), args.topic)
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    main(sys.argv[1:])
