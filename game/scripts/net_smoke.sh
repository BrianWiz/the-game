#!/usr/bin/env bash
# Starts a dedicated server + 2 clients headlessly and asserts that every peer
# sees both players (spawner + synchronizer working) with no script errors.
set -euo pipefail
cd "$(dirname "$0")/.."
GODOT="${GODOT:-godot}"
PORT="${PORT:-$((20000 + RANDOM % 20000))}"
LOGS="$(mktemp -d)"
trap 'kill $(jobs -p) 2>/dev/null || true' EXIT

"$GODOT" --headless -- --server --port="$PORT" --debug-roster >"$LOGS/server.log" 2>&1 &
sleep 2
"$GODOT" --headless -- --connect="ws://127.0.0.1:$PORT" --debug-roster >"$LOGS/c1.log" 2>&1 &
"$GODOT" --headless -- --connect="ws://127.0.0.1:$PORT" --debug-roster >"$LOGS/c2.log" 2>&1 &
sleep 6
kill $(jobs -p) 2>/dev/null || true
wait 2>/dev/null || true

fail=0
for f in server c1 c2; do
  if grep -E "SCRIPT ERROR|ERROR:" "$LOGS/$f.log"; then
    echo "$f: errors in log" >&2; fail=1
  fi
  last=$(grep '^ROSTER' "$LOGS/$f.log" | tail -1 || true)
  count=$(sed -n 's/.*players=//p' <<<"$last" | tr ',' '\n' | grep -c . || true)
  echo "$f: ${last:-<no roster>}"
  if [[ "$count" -ne 2 ]]; then
    echo "$f: expected 2 players, saw $count" >&2; fail=1
  fi
done
if [[ $fail -ne 0 ]]; then
  echo "logs: $LOGS" >&2; exit 1
fi
rm -rf "$LOGS"
echo "multiplayer smoke passed"
