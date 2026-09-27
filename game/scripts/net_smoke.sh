#!/usr/bin/env bash
# Starts a dedicated server + 2 clients headlessly and asserts that every peer
# sees both players (spawner + synchronizer working) with no script errors.
# The server runs with --dev-insecure-auth, so clients join with unsigned "dev:" tickets;
# a third client presents a forged ticket and a fourth runs a different build; both must be
# turned away with a reason.
set -euo pipefail
cd "$(dirname "$0")/.."
GODOT="${GODOT:-godot}"
PORT="${PORT:-$((20000 + RANDOM % 20000))}"
LOGS="$(mktemp -d)"
trap 'kill $(jobs -p) 2>/dev/null || true' EXIT

"$GODOT" --headless -- --server --port="$PORT" --dev-insecure-auth --debug-roster >"$LOGS/server.log" 2>&1 &
sleep 2
url="ws://127.0.0.1:$PORT"
"$GODOT" --headless -- --connect="$url" --dev-insecure-auth --name=Alice --debug-roster >"$LOGS/c1.log" 2>&1 &
"$GODOT" --headless -- --connect="$url" --dev-insecure-auth --name=Bob --debug-roster >"$LOGS/c2.log" 2>&1 &
"$GODOT" --headless -- --connect="$url" --ticket=v1.forged.ticket >"$LOGS/intruder.log" 2>&1 &
"$GODOT" --headless -- --connect="$url" --dev-insecure-auth --build-version=stale >"$LOGS/stale.log" 2>&1 &
sleep 6
kill $(jobs -p) 2>/dev/null || true
wait 2>/dev/null || true

fail=0
for f in server c1 c2 intruder stale; do
  if grep -E "SCRIPT ERROR|ERROR:" "$LOGS/$f.log"; then
    echo "$f: errors in log" >&2; fail=1
  fi
done
for f in server c1 c2; do
  last=$(grep '^ROSTER' "$LOGS/$f.log" | tail -1 || true)
  count=$(sed -n 's/.*players=//p' <<<"$last" | tr ',' '\n' | grep -c . || true)
  echo "$f: ${last:-<no roster>}"
  if [[ "$count" -ne 2 ]]; then
    echo "$f: expected 2 players, saw $count" >&2; fail=1
  fi
done
for name in Alice Bob; do
  if ! grep -q "authenticated as $name" "$LOGS/server.log"; then
    echo "server: $name never authenticated" >&2; fail=1
  fi
done
if ! grep -q "Connection failed: invalid or expired join ticket" "$LOGS/intruder.log"; then
  echo "intruder: forged ticket was not rejected with a reason" >&2; fail=1
fi
if ! grep -q "Connection failed: This game is version stale but the server runs dev" "$LOGS/stale.log"; then
  echo "stale: mismatched build was not rejected with a version reason" >&2; fail=1
fi
if [[ $fail -ne 0 ]]; then
  echo "logs: $LOGS" >&2; exit 1
fi
rm -rf "$LOGS"
echo "multiplayer smoke passed"
