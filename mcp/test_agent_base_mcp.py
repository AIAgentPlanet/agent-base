import importlib.util
import json
import unittest
from pathlib import Path
from urllib.parse import urlparse
from unittest.mock import patch


MODULE_PATH = Path(__file__).with_name("agent_base_mcp.py")
SPEC = importlib.util.spec_from_file_location("agent_base_mcp", MODULE_PATH)
agent_base_mcp = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(agent_base_mcp)


class FakeResponse:
    def __init__(self, payload):
        self.payload = json.dumps(payload).encode("utf-8")

    def __enter__(self):
        return self

    def __exit__(self, *_args):
        return False

    def read(self):
        return self.payload


class AgentBaseMCPProxyTest(unittest.TestCase):
    def test_register_agent_proxies_to_agent_service(self):
        captured = {}

        def fake_urlopen(request, timeout):
            captured["url"] = request.full_url
            captured["method"] = request.get_method()
            captured["body"] = json.loads(request.data.decode("utf-8"))
            captured["timeout"] = timeout
            return FakeResponse({"id": "agt_000001"})

        with patch.object(agent_base_mcp, "urlopen", fake_urlopen):
            result = agent_base_mcp.call_tool(
                "agent_base_register_agent",
                {
                    "user_id": "user_a",
                    "type": "hermes",
                    "identity": "https://a.example/.well-known/agent.json",
                    "connection_mode": "mcp",
                },
            )

        self.assertEqual(captured["url"], "http://localhost:8090/api/v1/agents")
        self.assertEqual(captured["method"], "POST")
        self.assertEqual(captured["body"]["identity"], "https://a.example/.well-known/agent.json")
        self.assertIn("agt_000001", result["content"][0]["text"])

    def test_next_delivery_uses_connection_token_header(self):
        captured = {}

        def fake_urlopen(request, timeout):
            captured["url"] = request.full_url
            captured["method"] = request.get_method()
            captured["token"] = request.get_header("X-agent-connection-token")
            captured["timeout"] = timeout
            return FakeResponse({"id": "deliv_000001", "type": "turn.available"})

        with patch.object(agent_base_mcp, "urlopen", fake_urlopen):
            result = agent_base_mcp.call_tool(
                "agent_base_next_delivery",
                {"connection_id": "conn_000001", "connection_token": "act_secret"},
            )

        self.assertEqual(captured["url"], "http://localhost:8090/api/v1/connections/conn_000001/deliveries/next")
        self.assertEqual(captured["method"], "GET")
        self.assertEqual(captured["token"], "act_secret")
        self.assertIn("deliv_000001", result["content"][0]["text"])

    def test_submit_message_uses_bearer_token(self):
        captured = {}

        def fake_urlopen(request, timeout):
            captured["url"] = request.full_url
            captured["method"] = request.get_method()
            captured["authorization"] = request.get_header("Authorization")
            captured["body"] = json.loads(request.data.decode("utf-8"))
            captured["timeout"] = timeout
            return FakeResponse({"id": "msg_000001"})

        with patch.object(agent_base_mcp, "urlopen", fake_urlopen):
            result = agent_base_mcp.call_tool(
                "agent_base_submit_message",
                {
                    "session_id": "ses_000001",
                    "participant_id": "par_000001",
                    "turn_id": "turn_000001",
                    "content": "hello",
                    "session_token": "ast_secret",
                },
            )

        self.assertEqual(captured["url"], "http://localhost:8090/api/v1/sessions/ses_000001/messages")
        self.assertEqual(captured["method"], "POST")
        self.assertEqual(captured["authorization"], "Bearer ast_secret")
        self.assertEqual(captured["body"]["type"], "message")
        self.assertIn("msg_000001", result["content"][0]["text"])

    def test_debate_end_to_end_smoke_over_mcp_tools(self):
        service = FakeAgentService()

        with patch.object(agent_base_mcp, "urlopen", service.urlopen):
            agent_a = tool_json(
                "agent_base_register_agent",
                {
                    "user_id": "user_a",
                    "type": "hermes",
                    "identity": "https://a.example/.well-known/agent.json",
                    "display_name": "User A Hermes",
                    "connection_mode": "mcp",
                },
            )
            agent_b = tool_json(
                "agent_base_register_agent",
                {
                    "user_id": "user_b",
                    "type": "openclaw",
                    "identity": "https://b.example/.well-known/agent.json",
                    "display_name": "User B OpenClaw",
                    "connection_mode": "mcp",
                },
            )
            session = tool_json(
                "agent_base_create_session",
                {
                    "type": "debate",
                    "owner_user_id": "user_a",
                    "policy": {
                        "turn_policy": "alternate",
                        "max_turns": 2,
                        "allowed_message_types": ["argument"],
                        "audit_required": True,
                    },
                    "metadata": {"topic": "MCP adapter 是否足以承载个人 agent 辩论"},
                },
            )
            participant_a = tool_json(
                "agent_base_join_session",
                {
                    "session_id": session["id"],
                    "user_id": "user_a",
                    "agent_id": agent_a["id"],
                    "role": "affirmative",
                },
            )
            participant_b = tool_json(
                "agent_base_join_session",
                {
                    "session_id": session["id"],
                    "user_id": "user_b",
                    "agent_id": agent_b["id"],
                    "role": "negative",
                },
            )
            connection_a = tool_json("agent_base_register_connection", {"agent_id": agent_a["id"], "mode": "mcp"})
            connection_b = tool_json("agent_base_register_connection", {"agent_id": agent_b["id"], "mode": "mcp"})

            delivery_a = tool_json(
                "agent_base_next_delivery",
                {"connection_id": connection_a["id"], "connection_token": connection_a["connection_token"]},
            )
            self.assertEqual(delivery_a["participant_id"], participant_a["id"])
            tool_json(
                "agent_base_ack_delivery",
                {
                    "connection_id": connection_a["id"],
                    "delivery_id": delivery_a["id"],
                    "connection_token": connection_a["connection_token"],
                },
            )
            turn_a = tool_json(
                "agent_base_get_next_turn",
                {
                    "session_id": session["id"],
                    "participant_id": participant_a["id"],
                    "session_token": participant_a["session_token"],
                },
            )
            tool_json(
                "agent_base_submit_message",
                {
                    "session_id": session["id"],
                    "participant_id": participant_a["id"],
                    "turn_id": turn_a["id"],
                    "type": "argument",
                    "content": "正方：MCP adapter 可以把 agent 接入统一 session。",
                    "session_token": participant_a["session_token"],
                },
            )

            delivery_b = tool_json(
                "agent_base_next_delivery",
                {"connection_id": connection_b["id"], "connection_token": connection_b["connection_token"]},
            )
            self.assertEqual(delivery_b["participant_id"], participant_b["id"])
            tool_json(
                "agent_base_ack_delivery",
                {
                    "connection_id": connection_b["id"],
                    "delivery_id": delivery_b["id"],
                    "connection_token": connection_b["connection_token"],
                },
            )
            turn_b = tool_json(
                "agent_base_get_next_turn",
                {
                    "session_id": session["id"],
                    "participant_id": participant_b["id"],
                    "session_token": participant_b["session_token"],
                },
            )
            tool_json(
                "agent_base_submit_message",
                {
                    "session_id": session["id"],
                    "participant_id": participant_b["id"],
                    "turn_id": turn_b["id"],
                    "type": "argument",
                    "content": "反方：MVP 可以验证流程，但生产仍需确认和审计 UI。",
                    "session_token": participant_b["session_token"],
                },
            )

            messages = tool_json("agent_base_list_messages", {"session_id": session["id"]})
            completed = tool_json("agent_base_get_session", {"session_id": session["id"]})

        self.assertEqual([message["participant_id"] for message in messages], [participant_a["id"], participant_b["id"]])
        self.assertEqual(completed["status"], "completed")
        self.assertEqual(service.call_count["POST /api/v1/sessions/" + session["id"] + "/messages"], 2)


def tool_json(name, arguments):
    result = agent_base_mcp.call_tool(name, arguments)
    return json.loads(result["content"][0]["text"])


class FakeAgentService:
    def __init__(self):
        self.next_id = 0
        self.agents = {}
        self.sessions = {}
        self.participants = {}
        self.connections = {}
        self.turns = {}
        self.session_turns = {}
        self.messages = {}
        self.session_messages = {}
        self.deliveries = {}
        self.call_count = {}

    def urlopen(self, request, timeout):
        del timeout
        parsed = urlparse(request.full_url)
        method = request.get_method()
        path = parsed.path
        key = f"{method} {path}"
        self.call_count[key] = self.call_count.get(key, 0) + 1
        body = {}
        if request.data:
            body = json.loads(request.data.decode("utf-8"))
        return FakeResponse(self.handle(method, path, request, body))

    def handle(self, method, path, request, body):
        if method == "POST" and path == "/api/v1/agents":
            agent = {
                "id": self.id("agt"),
                "user_id": body["user_id"],
                "type": body.get("type", "custom"),
                "identity": body["identity"],
                "display_name": body.get("display_name", body["identity"]),
                "connection_mode": body.get("connection_mode", "mcp"),
            }
            self.agents[agent["id"]] = agent
            return agent
        if method == "POST" and path == "/api/v1/sessions":
            session = {
                "id": self.id("ses"),
                "type": body.get("type", "debate"),
                "owner_user_id": body["owner_user_id"],
                "status": "waiting_participants",
                "policy": body.get("policy", {}),
                "metadata": body.get("metadata", {}),
                "participants": [],
            }
            session["policy"].setdefault("max_turns", 2)
            session["policy"].setdefault("allowed_message_types", ["argument", "message"])
            self.sessions[session["id"]] = session
            self.session_turns[session["id"]] = []
            self.session_messages[session["id"]] = []
            return session
        if method == "GET" and path.startswith("/api/v1/sessions/") and path.count("/") == 4:
            session_id = path.rsplit("/", 1)[1]
            return self.sessions[session_id]
        if method == "POST" and path.endswith("/participants"):
            session_id = path.split("/")[4]
            agent = self.agents[body["agent_id"]]
            participant = {
                "id": self.id("par"),
                "session_id": session_id,
                "user_id": body["user_id"],
                "agent_id": agent["id"],
                "agent_identity": agent["identity"],
                "role": body.get("role", "speaker"),
                "session_token": self.id("ast"),
            }
            self.participants[participant["id"]] = participant
            session = self.sessions[session_id]
            public = dict(participant)
            public.pop("session_token")
            session["participants"].append(public)
            if len(session["participants"]) == 2:
                session["status"] = "active"
                self.rebuild_turns(session)
            return participant
        if method == "POST" and path == "/api/v1/connections":
            agent = self.agents[body["agent_id"]]
            connection = {
                "id": self.id("conn"),
                "agent_id": agent["id"],
                "user_id": agent["user_id"],
                "mode": body.get("mode", agent["connection_mode"]),
                "connection_token": self.id("act"),
            }
            self.connections[connection["id"]] = connection
            return connection
        if method == "GET" and path.endswith("/deliveries/next"):
            connection_id = path.split("/")[4]
            self.require_connection_token(request, connection_id)
            return self.next_delivery(connection_id)
        if method == "POST" and path.endswith("/ack"):
            parts = path.split("/")
            connection_id = parts[4]
            delivery_id = parts[6]
            self.require_connection_token(request, connection_id)
            delivery = self.deliveries[delivery_id]
            delivery["status"] = "acked"
            return delivery
        if method == "GET" and path.endswith("/next-turn"):
            parts = path.split("/")
            session_id = parts[4]
            participant_id = parts[6]
            self.require_session_token(request, participant_id)
            for turn_id in self.session_turns[session_id]:
                turn = self.turns[turn_id]
                if turn["status"] == "open":
                    if turn["participant_id"] != participant_id:
                        raise AssertionError("not your turn")
                    return turn
            raise AssertionError("no turn available")
        if method == "POST" and path.endswith("/messages"):
            session_id = path.split("/")[4]
            participant_id = body["participant_id"]
            self.require_session_token(request, participant_id)
            message = {
                "id": self.id("msg"),
                "session_id": session_id,
                "participant_id": participant_id,
                "turn_id": body["turn_id"],
                "type": body["type"],
                "content": body["content"],
            }
            self.messages[message["id"]] = message
            self.session_messages[session_id].append(message["id"])
            self.turns[body["turn_id"]]["status"] = "submitted"
            if all(self.turns[turn_id]["status"] != "open" for turn_id in self.session_turns[session_id]):
                self.sessions[session_id]["status"] = "completed"
            return message
        if method == "GET" and path.endswith("/messages"):
            session_id = path.split("/")[4]
            return [self.messages[message_id] for message_id in self.session_messages[session_id]]
        raise AssertionError(f"unhandled request: {method} {path}")

    def rebuild_turns(self, session):
        participants = session["participants"]
        for index in range(session["policy"]["max_turns"]):
            participant = participants[index % len(participants)]
            turn = {
                "id": self.id("turn"),
                "session_id": session["id"],
                "participant_id": participant["id"],
                "index": index + 1,
                "phase": "argument",
                "status": "open",
            }
            self.turns[turn["id"]] = turn
            self.session_turns[session["id"]].append(turn["id"])

    def next_delivery(self, connection_id):
        connection = self.connections[connection_id]
        for delivery in self.deliveries.values():
            if delivery["connection_id"] == connection_id and delivery["status"] == "pending":
                return delivery
        for session_id, turn_ids in self.session_turns.items():
            for turn_id in turn_ids:
                turn = self.turns[turn_id]
                if turn["status"] != "open":
                    continue
                participant = self.participants[turn["participant_id"]]
                if participant["agent_id"] != connection["agent_id"]:
                    continue
                if self.delivery_exists(connection_id, turn_id):
                    continue
                delivery = {
                    "id": self.id("deliv"),
                    "connection_id": connection_id,
                    "agent_id": connection["agent_id"],
                    "session_id": session_id,
                    "participant_id": participant["id"],
                    "turn_id": turn_id,
                    "type": "turn.available",
                    "status": "pending",
                    "payload": {"session_id": session_id, "participant_id": participant["id"], "turn_id": turn_id},
                }
                self.deliveries[delivery["id"]] = delivery
                return delivery
        raise AssertionError("no delivery")

    def delivery_exists(self, connection_id, turn_id):
        return any(
            delivery["connection_id"] == connection_id and delivery["turn_id"] == turn_id
            for delivery in self.deliveries.values()
        )

    def require_connection_token(self, request, connection_id):
        token = request.get_header("X-agent-connection-token")
        if token != self.connections[connection_id]["connection_token"]:
            raise AssertionError("bad connection token")

    def require_session_token(self, request, participant_id):
        token = request.get_header("Authorization")
        if token != "Bearer " + self.participants[participant_id]["session_token"]:
            raise AssertionError("bad session token")

    def id(self, prefix):
        self.next_id += 1
        return f"{prefix}_{self.next_id:06d}"


if __name__ == "__main__":
    unittest.main()
