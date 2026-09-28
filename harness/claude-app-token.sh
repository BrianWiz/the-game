#!/usr/bin/env bash
# Prints an installation token for the Claude GitHub App (claude[bot]), for repos without
# their own agent App. It trades the job's GitHub OIDC token (needs `id-token: write`) with
# Anthropic, the same way anthropics/claude-code-action does. That exchange endpoint is
# internal to the action and may change; prefer a dedicated App (see harness/README.md).
#
# The token can write contents, pull requests and issues, and expires after an hour.
# Revoke it when done: gh api -X DELETE installation/token
set -euo pipefail
: "${ACTIONS_ID_TOKEN_REQUEST_URL:?needs id-token: write in a GitHub Actions job}"

oidc="$(curl -fsS -H "Authorization: bearer $ACTIONS_ID_TOKEN_REQUEST_TOKEN" \
  "$ACTIONS_ID_TOKEN_REQUEST_URL&audience=claude-code-github-action" | jq -r .value)"
echo "::add-mask::$oidc"

resp="$(curl -sS -X POST -H "Authorization: Bearer $oidc" -w '\n%{http_code}' \
  https://api.anthropic.com/api/github/github-app-token-exchange)"
code="$(tail -n 1 <<<"$resp")"
body="$(sed '$d' <<<"$resp")"
token="$(jq -r '.token // .app_token // empty' <<<"$body" 2>/dev/null || true)"
if [[ "$code" != 200 || -z "$token" ]]; then
  echo "Claude app token exchange failed (HTTP $code): $(jq -r '.error.message // .message // .' <<<"$body" 2>/dev/null | head -c 300)" >&2
  echo "Is the Claude GitHub App installed on this repo (https://github.com/apps/claude)?" >&2
  exit 1
fi
echo "::add-mask::$token"
printf '%s\n' "$token"
