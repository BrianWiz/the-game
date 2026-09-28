#!/usr/bin/env bash
# pi adapter. Usage: pi.sh PROMPT_FILE LOG_FILE CONTINUE(0|1)
# Env: HARNESS_OUT (required), HARNESS_MODEL. Auth: a persisted pi auth.json login.
set -euo pipefail
prompt="$1" log="$2" cont="$3"

args=(-p --session-dir "$HARNESS_OUT/pi-sessions")
[[ -n "${HARNESS_MODEL:-}" ]] && args+=(--model "$HARNESS_MODEL")
[[ "$cont" == 1 ]] && args+=(--continue)

set +e
pi "${args[@]}" -- "$(cat "$prompt")" </dev/null 2>&1 | tee "$log"
status="${PIPESTATUS[0]}"
set -e

cp "$log" "$HARNESS_OUT/last-message.md"
exit "$status"
