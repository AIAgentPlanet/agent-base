#!/bin/bash
# ATH v0.1 gateway flow test.
# The agent identity document must already be available at AGENT_ID over public HTTPS.

set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
AGENT_ID="${AGENT_ID:-}"
AGENT_PRIVATE_KEY="${AGENT_PRIVATE_KEY:-}"
REDIRECT_URI="${REDIRECT_URI:-http://localhost:3000/callback}"
EPHEMERAL_PRIVATE_KEY="$(mktemp)"
trap 'rm -f "$EPHEMERAL_PRIVATE_KEY"' EXIT

if [ -z "$AGENT_ID" ] || [ -z "$AGENT_PRIVATE_KEY" ]; then
  echo "Usage: AGENT_ID=https://agent.example/.well-known/agent.json AGENT_PRIVATE_KEY=agent-private.pem $0"
  echo "The identity document must publish the matching P-256 public key."
  exit 2
fi

make_attestation() {
  local audience="$1"
  AGENT_ID="$AGENT_ID" AGENT_PRIVATE_KEY="$AGENT_PRIVATE_KEY" AUDIENCE="$audience" python3 <<'PY'
import os, time, uuid, jwt
from cryptography.hazmat.primitives import serialization

with open(os.environ["AGENT_PRIVATE_KEY"], "rb") as f:
    private_key = serialization.load_pem_private_key(f.read(), password=None)

now = int(time.time())
claims = {
    "iss": os.environ["AGENT_ID"],
    "sub": os.environ["AGENT_ID"],
    "aud": os.environ["AUDIENCE"],
    "iat": now,
    "exp": now + 300,
    "jti": str(uuid.uuid4()),
}
print(jwt.encode(claims, private_key, algorithm="ES256"))
PY
}

echo "========== 1. Discovery =========="
curl -fsS "$BASE_URL/.well-known/ath.json" | python3 -m json.tool

echo "========== 2. Register =========="
REGISTER_ATTESTATION="$(make_attestation "$BASE_URL/api/v1/ath/agents/register")"
REGISTER_RESP="$(curl -fsS -X POST "$BASE_URL/api/v1/ath/agents/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"agent_id\": \"$AGENT_ID\",
    \"agent_attestation\": \"$REGISTER_ATTESTATION\",
    \"developer\": {\"name\": \"ATH Test\", \"id\": \"ath-test\"},
    \"redirect_uris\": [\"$REDIRECT_URI\"],
    \"requested_providers\": [
      {\"provider_id\": \"user-service\", \"scopes\": [\"user:read\"]}
    ],
    \"purpose\": \"ATH v0.1 compatibility test\"
  }")"
echo "$REGISTER_RESP" | python3 -m json.tool

CLIENT_ID="$(echo "$REGISTER_RESP" | python3 -c 'import sys,json; print(json.load(sys.stdin)["client_id"])')"
CLIENT_SECRET="$(echo "$REGISTER_RESP" | python3 -c 'import sys,json; print(json.load(sys.stdin)["client_secret"])')"

echo "========== 3. Mutual identity handshake =========="
SERVER_DID_DOC="$(curl -fsS "$BASE_URL/.well-known/did.json")"
CLIENT_NONCE="$(python3 -c 'import base64,secrets; print(base64.urlsafe_b64encode(secrets.token_bytes(32)).rstrip(b\"=\").decode())')"
CLIENT_EPHEMERAL_KEY="$(EPHEMERAL_PRIVATE_KEY="$EPHEMERAL_PRIVATE_KEY" python3 <<'PY'
import base64, os
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec

private_key = ec.generate_private_key(ec.SECP256R1())
with open(os.environ["EPHEMERAL_PRIVATE_KEY"], "wb") as f:
    f.write(private_key.private_bytes(
        serialization.Encoding.PEM,
        serialization.PrivateFormat.PKCS8,
        serialization.NoEncryption(),
    ))
public_bytes = private_key.public_key().public_bytes(
    serialization.Encoding.X962,
    serialization.PublicFormat.UncompressedPoint,
)
print(base64.urlsafe_b64encode(public_bytes).rstrip(b"=").decode())
PY
)"
HANDSHAKE_TIMESTAMP="$(date +%s)"
HANDSHAKE_RESP="$(curl -fsS -X POST "$BASE_URL/api/v1/ath/handshakes" \
  -H "Content-Type: application/json" \
  -d "{
    \"client_id\": \"$CLIENT_ID\",
    \"client_did\": \"$AGENT_ID\",
    \"versions\": [\"0.1\"],
    \"capabilities\": [\"ES256\", \"SHA-256\", \"OAuth2\", \"PKCE-S256\", \"ECDH-P256\", \"HKDF-SHA256\", \"HMAC-SHA256\"],
    \"nonce\": \"$CLIENT_NONCE\",
    \"ephemeral_key\": \"$CLIENT_EPHEMERAL_KEY\",
    \"timestamp\": $HANDSHAKE_TIMESTAMP
  }")"
echo "$HANDSHAKE_RESP" | python3 -m json.tool

HANDSHAKE_ID="$(echo "$HANDSHAKE_RESP" | python3 -c 'import sys,json; print(json.load(sys.stdin)["handshake_id"])')"
PROOF_TIMESTAMP="$(date +%s)"
CLIENT_PROOF="$(HANDSHAKE_RESP="$HANDSHAKE_RESP" SERVER_DID_DOC="$SERVER_DID_DOC" AGENT_ID="$AGENT_ID" AGENT_PRIVATE_KEY="$AGENT_PRIVATE_KEY" PROOF_TIMESTAMP="$PROOF_TIMESTAMP" python3 <<'PY'
import base64, json, os
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec

handshake = json.loads(os.environ["HANDSHAKE_RESP"])
did_document = json.loads(os.environ["SERVER_DID_DOC"])
published_key = did_document["verificationMethod"][0]["publicKeyPem"]
if published_key != handshake["server_public_key"]:
    raise ValueError("server challenge key does not match DID document")
server_payload = {
    "type": "server_challenge",
    "handshake_id": handshake["handshake_id"],
    "client_did": os.environ["AGENT_ID"],
    "server_did": handshake["server_did"],
    "client_nonce": handshake["client_nonce"],
    "server_nonce": handshake["server_nonce"],
    "client_ephemeral_key": handshake["client_ephemeral_key"],
    "server_ephemeral_key": handshake["server_ephemeral_key"],
    "version": handshake["version"],
    "timestamp": handshake["timestamp"],
}
server_public_key = serialization.load_pem_public_key(
    handshake["server_public_key"].encode()
)
server_public_key.verify(
    base64.urlsafe_b64decode(handshake["signature"] + "=="),
    json.dumps(server_payload, separators=(",", ":")).encode(),
    ec.ECDSA(hashes.SHA256()),
)
payload = {
    "type": "client_identity_proof",
    "handshake_id": handshake["handshake_id"],
    "client_did": os.environ["AGENT_ID"],
    "server_did": handshake["server_did"],
    "server_nonce": handshake["server_nonce"],
    "client_ephemeral_key": handshake["client_ephemeral_key"],
    "server_ephemeral_key": handshake["server_ephemeral_key"],
    "version": handshake["version"],
    "timestamp": int(os.environ["PROOF_TIMESTAMP"]),
}
with open(os.environ["AGENT_PRIVATE_KEY"], "rb") as f:
    private_key = serialization.load_pem_private_key(f.read(), password=None)
signature = private_key.sign(
    json.dumps(payload, separators=(",", ":")).encode(),
    ec.ECDSA(hashes.SHA256()),
)
print(base64.urlsafe_b64encode(signature).rstrip(b"=").decode())
PY
)"
PROOF_RESP="$(curl -fsS -X POST "$BASE_URL/api/v1/ath/handshakes/$HANDSHAKE_ID/proof" \
  -H "Content-Type: application/json" \
  -d "{
    \"client_id\": \"$CLIENT_ID\",
    \"signature\": \"$CLIENT_PROOF\",
    \"timestamp\": $PROOF_TIMESTAMP
  }")"
echo "$PROOF_RESP" | python3 -m json.tool

SESSION_KEY="$(HANDSHAKE_RESP="$HANDSHAKE_RESP" AGENT_ID="$AGENT_ID" EPHEMERAL_PRIVATE_KEY="$EPHEMERAL_PRIVATE_KEY" python3 <<'PY'
import base64, hashlib, json, os
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.kdf.hkdf import HKDF

handshake = json.loads(os.environ["HANDSHAKE_RESP"])
with open(os.environ["EPHEMERAL_PRIVATE_KEY"], "rb") as f:
    private_key = serialization.load_pem_private_key(f.read(), password=None)
server_public_key = ec.EllipticCurvePublicKey.from_encoded_point(
    ec.SECP256R1(),
    base64.urlsafe_b64decode(handshake["server_ephemeral_key"] + "=="),
)
shared_secret = private_key.exchange(ec.ECDH(), server_public_key)
client_nonce = base64.urlsafe_b64decode(handshake["client_nonce"] + "==")
server_nonce = base64.urlsafe_b64decode(handshake["server_nonce"] + "==")
salt = hashlib.sha256(client_nonce + server_nonce).digest()
info = {
    "protocol": "ATH-ECDH-P256-HKDF-SHA256",
    "handshake_id": handshake["handshake_id"],
    "client_did": os.environ["AGENT_ID"],
    "server_did": handshake["server_did"],
    "version": handshake["version"],
}
key = HKDF(
    algorithm=hashes.SHA256(), length=32, salt=salt,
    info=json.dumps(info, separators=(",", ":")).encode(),
).derive(shared_secret)
print(base64.urlsafe_b64encode(key).rstrip(b"=").decode())
PY
)"

echo "========== 4. Authorize =========="
STATE="$(python3 -c 'import secrets; print(secrets.token_urlsafe(24))')"
AUTHORIZE_ATTESTATION="$(make_attestation "$BASE_URL/api/v1/ath/authorize")"
AUTH_RESP="$(curl -fsS -X POST "$BASE_URL/api/v1/ath/authorize" \
  -H "Content-Type: application/json" \
  -d "{
    \"client_id\": \"$CLIENT_ID\",
    \"handshake_id\": \"$HANDSHAKE_ID\",
    \"agent_attestation\": \"$AUTHORIZE_ATTESTATION\",
    \"provider_id\": \"user-service\",
    \"scopes\": [\"user:read\"],
    \"state\": \"$STATE\",
    \"user_redirect_uri\": \"$REDIRECT_URI\"
  }")"
echo "$AUTH_RESP" | python3 -m json.tool

AUTH_URL="$(echo "$AUTH_RESP" | python3 -c 'import sys,json; print(json.load(sys.stdin)["authorization_url"])')"
SESSION_ID="$(echo "$AUTH_RESP" | python3 -c 'import sys,json; print(json.load(sys.stdin)["ath_session_id"])')"

echo "========== 5. User consent =========="
LOGIN_RESP="$(curl -fsS -X POST "$BASE_URL/api/v1/users/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"123456"}')"
APP_JWT="$(echo "$LOGIN_RESP" | python3 -c 'import sys,json; print(json.load(sys.stdin).get("data",{}).get("token",""))')"
CODE_RESP="$(curl -sS -o /dev/null -w "%{redirect_url}" -H "Authorization: Bearer $APP_JWT" "$AUTH_URL")"
AUTH_CODE="$(REDIRECT_URL="$CODE_RESP" python3 -c 'import os; from urllib.parse import urlparse,parse_qs; print(parse_qs(urlparse(os.environ["REDIRECT_URL"]).query)["code"][0])')"

echo "========== 6. Token exchange =========="
TOKEN_ATTESTATION="$(make_attestation "$BASE_URL/api/v1/ath/token")"
TOKEN_RESP="$(curl -fsS -X POST "$BASE_URL/api/v1/ath/token" \
  -H "Content-Type: application/json" \
  -d "{
    \"grant_type\": \"authorization_code\",
    \"client_id\": \"$CLIENT_ID\",
    \"client_secret\": \"$CLIENT_SECRET\",
    \"agent_attestation\": \"$TOKEN_ATTESTATION\",
    \"code\": \"$AUTH_CODE\",
    \"ath_session_id\": \"$SESSION_ID\"
  }")"
echo "$TOKEN_RESP" | python3 -m json.tool

echo "========== 7. Integrity-protected proxy call =========="
ACCESS_TOKEN="$(echo "$TOKEN_RESP" | python3 -c 'import sys,json; print(json.load(sys.stdin)["access_token"])')"
REQUEST_TIMESTAMP="$(date +%s)"
REQUEST_NONCE="$(python3 -c 'import base64,secrets; print(base64.urlsafe_b64encode(secrets.token_bytes(16)).rstrip(b\"=\").decode())')"
REQUEST_SIGNATURE="$(ACCESS_TOKEN="$ACCESS_TOKEN" HANDSHAKE_ID="$HANDSHAKE_ID" SESSION_KEY="$SESSION_KEY" REQUEST_TIMESTAMP="$REQUEST_TIMESTAMP" REQUEST_NONCE="$REQUEST_NONCE" python3 <<'PY'
import base64, hashlib, hmac, json, os

token_payload = os.environ["ACCESS_TOKEN"].split(".")[1]
token_payload += "=" * (-len(token_payload) % 4)
jti = json.loads(base64.urlsafe_b64decode(token_payload))["jti"]
payload = {
    "type": "ath_request_integrity",
    "handshake_id": os.environ["HANDSHAKE_ID"],
    "token_jti": jti,
    "provider": "user-service",
    "method": "GET",
    "path": "/api/v1/users/profile",
    "body_sha256": hashlib.sha256(b"").hexdigest(),
    "timestamp": int(os.environ["REQUEST_TIMESTAMP"]),
    "nonce": os.environ["REQUEST_NONCE"],
}
key = base64.urlsafe_b64decode(os.environ["SESSION_KEY"] + "==")
signature = hmac.new(
    key, json.dumps(payload, separators=(",", ":")).encode(), hashlib.sha256
).digest()
print(base64.urlsafe_b64encode(signature).rstrip(b"=").decode())
PY
)"
curl -fsS -X POST "$BASE_URL/api/v1/ath/proxy" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -d "{
    \"provider\": \"user-service\",
    \"method\": \"GET\",
    \"path\": \"/api/v1/users/profile\",
    \"request_timestamp\": $REQUEST_TIMESTAMP,
    \"request_nonce\": \"$REQUEST_NONCE\",
    \"request_signature\": \"$REQUEST_SIGNATURE\"
  }" | python3 -m json.tool

echo "========== 8. Audit records =========="
curl -fsS -X POST "$BASE_URL/api/v1/ath/audit/query" \
  -H "Content-Type: application/json" \
  -d "{
    \"client_id\": \"$CLIENT_ID\",
    \"client_secret\": \"$CLIENT_SECRET\",
    \"handshake_id\": \"$HANDSHAKE_ID\",
    \"limit\": 100
  }" | python3 -m json.tool

echo "========== 9. Audit chain verification =========="
curl -fsS -X POST "$BASE_URL/api/v1/ath/audit/verify" \
  -H "Content-Type: application/json" \
  -d "{
    \"client_id\": \"$CLIENT_ID\",
    \"client_secret\": \"$CLIENT_SECRET\"
  }" | python3 -m json.tool

echo "========== 10. Public audit head =========="
curl -fsS "$BASE_URL/.well-known/ath-audit-head.json" | python3 -m json.tool

echo "========== 11. External anchor status =========="
curl -fsS -X POST "$BASE_URL/api/v1/ath/audit/anchor/status" \
  -H "Content-Type: application/json" \
  -d "{
    \"client_id\": \"$CLIENT_ID\",
    \"client_secret\": \"$CLIENT_SECRET\"
  }" | python3 -m json.tool

echo "========== ATH v0.1 flow complete =========="
