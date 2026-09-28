#!/usr/bin/env bash
# Codex CLI adapter. Usage: codex.sh PROMPT_FILE LOG_FILE CONTINUE(0|1)
# Env: HARNESS_OUT (required), HARNESS_MODEL. Auth: a persisted ~/.codex/auth.json login.
set -euo pipefail
prompt="$1" log="$2" cont="$3"
session_file="$HARNESS_OUT/codex-session"

opts=(--json --dangerously-bypass-approvals-and-sandbox -o "$HARNESS_OUT/last-message.md")
[[ -n "${HARNESS_MODEL:-}" ]] && opts+=(-m "$HARNESS_MODEL")

set +e
if [[ "$cont" == 1 && -s "$session_file" ]]; then
  codex exec resume "${opts[@]}" "$(cat "$session_file")" - <"$prompt" >"$log" 2>&1
else
  codex exec "${opts[@]}" - <"$prompt" >"$log" 2>&1
fi
status=$?
set -e

jq -r 'select(.type == "thread.started") | .thread_id' "$log" 2>/dev/null | head -1 >"$session_file.new" || true
[[ -s "$session_file.new" ]] && mv "$session_file.new" "$session_file"
rm -f "$session_file.new"
cat "$HARNESS_OUT/last-message.md" 2>/dev/null || true
exit "$status"
