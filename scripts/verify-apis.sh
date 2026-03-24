#!/usr/bin/env bash
# End-to-end HTTP checks for Nebula Strike Nakama RPCs (same paths as nebula-strike nakamaClient).
# Requires: curl, jq, node (for JSON-RPC body encoding), running Nakama on 7350 (or nginx on 80).
# Optional: NAKAMA_HOST, NAKAMA_PORT, NAKAMA_SERVER_KEY — otherwise loads repo .env if present.
set -euo pipefail

_REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ -f "${_REPO_ROOT}/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${_REPO_ROOT}/.env"
  set +a
fi

HOST="${NAKAMA_HOST:-127.0.0.1}"
PORT="${NAKAMA_PORT:-7350}"
KEY="${NAKAMA_SERVER_KEY:-nebula-strike-dev-key}"
BASE="http://${HOST}:${PORT}"

b64_key() {
  if echo -n "${KEY}:" | base64 -w0 >/dev/null 2>&1; then
    echo -n "${KEY}:" | base64 -w0
  else
    echo -n "${KEY}:" | base64
  fi
}

echo "== 1) Device authenticate (HTTP) =="
BASIC="$(b64_key)"
DEVICE_ID="e2e-verify-$(date +%s)-$$"
UNAME="e2e-$(date +%s)-$$"
AUTH_JSON="$(curl -fsS "${BASE}/v2/account/authenticate/device?create=true&username=${UNAME}" \
  -H "Authorization: Basic ${BASIC}" \
  -H "Content-Type: application/json" \
  -d "{\"id\":\"${DEVICE_ID}\"}")"

TOKEN="$(echo "$AUTH_JSON" | jq -r '.token')"
if [[ -z "$TOKEN" || "$TOKEN" == "null" ]]; then
  echo "FAIL: no token. Response: $AUTH_JSON" >&2
  exit 1
fi
echo "OK session token received (${#TOKEN} chars)"

echo "== 2) RPC create_match =="
CM_BODY="$(node -e "console.log(JSON.stringify(JSON.stringify({mode:'classic'})))")"
CM_RESP="$(curl -fsS "${BASE}/v2/rpc/create_match" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "${CM_BODY}")"
echo "$CM_RESP" | jq .
MID="$(echo "$CM_RESP" | jq -r '.payload | fromjson | .matchId')"
if [[ -z "$MID" || "$MID" == "null" ]]; then
  echo "FAIL: create_match missing matchId" >&2
  exit 1
fi
echo "OK matchId=$MID"

echo "== 3) RPC get_leaderboard =="
LB_BODY="$(node -e "console.log(JSON.stringify(JSON.stringify({})))")"
LB_RESP="$(curl -fsS "${BASE}/v2/rpc/get_leaderboard" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "${LB_BODY}")"
echo "$LB_RESP" | jq .
echo "$LB_RESP" | jq -e '.payload | fromjson | .records | type == "array"' >/dev/null
echo "OK leaderboard payload shape"

echo "== 4) RPC get_match_history =="
MH_RESP="$(curl -fsS "${BASE}/v2/rpc/get_match_history" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "${LB_BODY}")"
echo "$MH_RESP" | jq .
echo "$MH_RESP" | jq -e '.payload | fromjson | .entries | type == "array"' >/dev/null
echo "OK match history payload shape"

echo ""
echo "All RPC checks passed. (WebSocket match flow requires browser/socket client.)"
