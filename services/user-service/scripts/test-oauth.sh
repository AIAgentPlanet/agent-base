#!/bin/bash
# OAuth 2.0 完整流程测试脚本
# 演示：登录 -> 创建客户端 -> 获取授权码 -> 换取 Token -> 获取用户信息 -> 注销 Token

set -e

BASE_URL="http://localhost:8080"
REDIRECT_URI="http://localhost:3000/callback"

echo "========== 1. 用户登录获取 JWT =========="
LOGIN_RESP=$(curl -s -X POST "$BASE_URL/api/v1/users/login" \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"123456"}')
JWT_TOKEN=$(echo "$LOGIN_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('token',''))")
echo "JWT Token: ${JWT_TOKEN:0:40}..."
echo ""

echo "========== 2. 创建 OAuth 客户端 =========="
UNIQUE_NAME="TestApp_$(date +%s)"
CLIENT_RESP=$(curl -s -X POST "$BASE_URL/api/v1/oauth/clients" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d "{\"name\":\"$UNIQUE_NAME\",\"redirect_uris\":[\"$REDIRECT_URI\"],\"allowed_grants\":[\"authorization_code\",\"refresh_token\"],\"allowed_scopes\":[\"profile\"]}")
echo "$CLIENT_RESP" | python3 -m json.tool

CLIENT_ID=$(echo "$CLIENT_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('client_id',''))")
CLIENT_SECRET=$(echo "$CLIENT_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('client_secret',''))")
echo "Client ID: $CLIENT_ID"
echo "Client Secret: ${CLIENT_SECRET:0:20}..."
echo ""

echo "========== 3. 获取授权码 (Authorization Code) =========="
# 注意：authorize 需要用户已登录（JWT），返回 302 重定向到 redirect_uri 并附带 code
AUTH_RESP=$(curl -s -o /dev/null -w "%{http_code}|%{redirect_url}" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  "$BASE_URL/api/v1/oauth/authorize?response_type=code&client_id=$CLIENT_ID&redirect_uri=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$REDIRECT_URI'))")&scope=profile&state=xyz123")

echo "响应状态: $(echo "$AUTH_RESP" | cut -d'|' -f1)"
REDIRECT_URL=$(echo "$AUTH_RESP" | cut -d'|' -f2)
echo "重定向地址: $REDIRECT_URL"

# 从重定向 URL 中提取 code
AUTH_CODE=$(python3 -c "from urllib.parse import urlparse, parse_qs; url='$REDIRECT_URL'; qs=parse_qs(urlparse(url).query); print(qs.get('code',[''])[0])")
echo "Authorization Code: $AUTH_CODE"
echo ""

echo "========== 4. 用授权码换取 Access Token =========="
TOKEN_RESP=$(curl -s -X POST "$BASE_URL/api/v1/oauth/token" \
  -H "Content-Type: application/json" \
  -d "{\"grant_type\":\"authorization_code\",\"code\":\"$AUTH_CODE\",\"redirect_uri\":\"$REDIRECT_URI\",\"client_id\":\"$CLIENT_ID\",\"client_secret\":\"$CLIENT_SECRET\"}")
echo "$TOKEN_RESP" | python3 -m json.tool

ACCESS_TOKEN=$(echo "$TOKEN_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('access_token',''))")
REFRESH_TOKEN=$(echo "$TOKEN_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('data',{}).get('refresh_token',''))")
echo ""

echo "========== 5. 用 Access Token 获取用户信息 =========="
USERINFO_RESP=$(curl -s "$BASE_URL/api/v1/oauth/userinfo" \
  -H "Authorization: Bearer $ACCESS_TOKEN")
echo "$USERINFO_RESP" | python3 -m json.tool
echo ""

echo "========== 6. 刷新 Token =========="
REFRESH_RESP=$(curl -s -X POST "$BASE_URL/api/v1/oauth/token" \
  -H "Content-Type: application/json" \
  -d "{\"grant_type\":\"refresh_token\",\"refresh_token\":\"$REFRESH_TOKEN\",\"client_id\":\"$CLIENT_ID\",\"client_secret\":\"$CLIENT_SECRET\"}")
echo "$REFRESH_RESP" | python3 -m json.tool
echo ""

echo "========== 7. 注销 Token =========="
REVOKE_RESP=$(curl -s -X POST "$BASE_URL/api/v1/oauth/revoke" \
  -H "Content-Type: application/json" \
  -d "{\"token\":\"$ACCESS_TOKEN\",\"token_type_hint\":\"access_token\"}")
echo "$REVOKE_RESP" | python3 -m json.tool
echo ""

echo "========== 测试完成 =========="
