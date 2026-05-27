#!/bin/bash
# ATH Protocol end-to-end test script
# This script demonstrates the full ATH flow: discover -> register -> authorize -> token -> proxy -> revoke

set -e

BASE_URL="http://localhost:8080"

echo "========== 0. Generate ES256 key pair for test agent =========="
# Generate private key
openssl ecparam -genkey -name prime256v1 -noout -out /tmp/ath-agent-private.pem 2>/dev/null
# Generate public key
openssl ec -in /tmp/ath-agent-private.pem -pubout -out /tmp/ath-agent-public.pem 2>/dev/null
# Create JWK-style public key PEM for registration
PUBLIC_KEY=$(cat /tmp/ath-agent-public.pem)
echo "Keys generated"
echo ""

echo "========== 1. Discovery =========="
curl -s "$BASE_URL/.well-known/ath.json" | python3 -m json.tool
echo ""

echo "========== 2. Generate attestation JWT =========="
# Create a simple ES256 JWT using openssl + python
python3 << 'PYEOF'
import jwt, time, uuid
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec

# Load private key
with open('/tmp/ath-agent-private.pem', 'rb') as f:
    private_key = serialization.load_pem_private_key(f.read(), password=None)

claims = {
    "agent_id": "did:ath:test-agent-001",
    "iss": "did:ath:test-agent-001",
    "sub": "did:ath:test-agent-001",
    "aud": "user-service",
    "iat": int(time.time()),
    "exp": int(time.time()) + 300,
    "jti": str(uuid.uuid4()),
}

token = jwt.encode(claims, private_key, algorithm="ES256")
with open('/tmp/ath-attestation.jwt', 'w') as f:
    f.write(token)
print("Attestation JWT created")
PYEOF

ATTESTATION=$(cat /tmp/ath-attestation.jwt)
echo ""

echo "========== 3. Register agent =========="
REGISTER_RESP=$(curl -s -X POST "$BASE_URL/api/v1/ath/agents/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"agent_id\": \"did:ath:test-agent-001\",
    \"attestation\": \"$ATTESTATION\",
    \"name\": \"Test Agent\",
    \"developer\": {\"name\": \"Test Org\", \"id\": \"dev-001\"},
    \"redirect_uris\": [\"http://localhost:3000/callback\"],
    \"providers\": [{\"provider_id\": \"user-service\", \"scopes\": [\"user:read\"]}],
    \"purpose\": \"Testing ATH protocol integration\"
  }")
echo "$REGISTER_RESP" | python3 -m json.tool

CLIENT_ID=$(echo "$REGISTER_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('client_id',''))")
CLIENT_SECRET=$(echo "$REGISTER_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('client_secret',''))")
echo "Client ID: $CLIENT_ID"
echo ""

echo "========== 4. Query agent status =========="
curl -s "$BASE_URL/api/v1/ath/agents/$CLIENT_ID" | python3 -m json.tool
echo ""

echo "========== 5. User login (get app JWT) =========="
LOGIN_RESP=$(curl -s -X POST "$BASE_URL/api/v1/users/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"123456"}')
APP_JWT=$(echo "$LOGIN_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('token',''))")
echo "App JWT obtained"
echo ""

echo "========== 6. ATH Authorize =========="
# Generate fresh attestation
python3 << 'PYEOF'
import jwt, time, uuid
from cryptography.hazmat.primitives import serialization
with open('/tmp/ath-agent-private.pem', 'rb') as f:
    private_key = serialization.load_pem_private_key(f.read(), password=None)
claims = {
    "agent_id": "did:ath:test-agent-001",
    "iss": "did:ath:test-agent-001",
    "sub": "did:ath:test-agent-001",
    "aud": "user-service",
    "iat": int(time.time()),
    "exp": int(time.time()) + 300,
    "jti": str(uuid.uuid4()),
}
token = jwt.encode(claims, private_key, algorithm="ES256")
with open('/tmp/ath-attestation2.jwt', 'w') as f:
    f.write(token)
PYEOF
ATTESTATION2=$(cat /tmp/ath-attestation2.jwt)

AUTH_RESP=$(curl -s -X POST "$BASE_URL/api/v1/ath/authorize" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ATTESTATION2" \
  -d "{
    \"client_id\": \"$CLIENT_ID\",
    \"provider_id\": \"user-service\",
    \"scopes\": [\"user:read\"],
    \"state\": \"xyz123\",
    \"redirect_uri\": \"http://localhost:3000/callback\"
  }")
echo "$AUTH_RESP" | python3 -m json.tool

AUTH_URL=$(echo "$AUTH_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('authorization_url',''))")
SESSION_ID=$(echo "$AUTH_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('ath_session_id',''))")
echo ""

echo "========== 7. User visits OAuth authorize (with app JWT) =========="
# Follow the redirect to get the code
CODE_RESP=$(curl -s -o /dev/null -w "%{http_code}|%{redirect_url}" \
  -H "Authorization: Bearer $APP_JWT" \
  "$AUTH_URL")
echo "Response: $CODE_RESP"
REDIRECT_URL=$(echo "$CODE_RESP" | cut -d'|' -f2)
AUTH_CODE=$(python3 -c "from urllib.parse import urlparse,parse_qs; url='$REDIRECT_URL'; qs=parse_qs(urlparse(url).query); print(qs.get('code',[''])[0])")
echo "Authorization Code: $AUTH_CODE"
echo ""

echo "========== 8. Exchange code for ATH token =========="
# Generate fresh attestation for token exchange
python3 << 'PYEOF'
import jwt, time, uuid
from cryptography.hazmat.primitives import serialization
with open('/tmp/ath-agent-private.pem', 'rb') as f:
    private_key = serialization.load_pem_private_key(f.read(), password=None)
claims = {
    "agent_id": "did:ath:test-agent-001",
    "iss": "did:ath:test-agent-001",
    "sub": "did:ath:test-agent-001",
    "aud": "user-service",
    "iat": int(time.time()),
    "exp": int(time.time()) + 300,
    "jti": str(uuid.uuid4()),
}
token = jwt.encode(claims, private_key, algorithm="ES256")
with open('/tmp/ath-attestation3.jwt', 'w') as f:
    f.write(token)
PYEOF
ATTESTATION3=$(cat /tmp/ath-attestation3.jwt)

TOKEN_RESP=$(curl -s -X POST "$BASE_URL/api/v1/ath/token" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ATTESTATION3" \
  -d "{
    \"grant_type\": \"authorization_code\",
    \"code\": \"$AUTH_CODE\",
    \"client_id\": \"$CLIENT_ID\",
    \"client_secret\": \"$CLIENT_SECRET\",
    \"ath_session_id\": \"$SESSION_ID\",
    \"redirect_uri\": \"http://localhost:3000/callback\"
  }")
echo "$TOKEN_RESP" | python3 -m json.tool

ATH_ACCESS_TOKEN=$(echo "$TOKEN_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('access_token',''))")
echo ""

echo "========== 9. Call protected API via ATH proxy =========="
curl -s -X POST "$BASE_URL/api/v1/ath/proxy" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ATH_ACCESS_TOKEN" \
  -d '{
    "provider": "user-service",
    "method": "GET",
    "path": "/api/v1/users/profile"
  }' | python3 -m json.tool
echo ""

echo "========== 10. Revoke ATH token =========="
curl -s -X POST "$BASE_URL/api/v1/ath/revoke" \
  -H "Content-Type: application/json" \
  -d "{
    \"token\": \"$ATH_ACCESS_TOKEN\",
    \"token_type_hint\": \"access_token\",
    \"client_id\": \"$CLIENT_ID\",
    \"client_secret\": \"$CLIENT_SECRET\"
  }" | python3 -m json.tool
echo ""

echo "========== ATH test complete =========="
