#!/usr/bin/env bash
# Export the web client and/or the dedicated server into ../build/.
# Usage: game/scripts/export.sh [web|server|all]   (needs Godot export templates)
set -euo pipefail
cd "$(dirname "$0")/.."
GODOT="${GODOT:-godot}"
target="${1:-all}"
"$GODOT" --headless --import >/dev/null 2>&1 || true
if [[ "$target" == web || "$target" == all ]]; then
  mkdir -p ../build/web
  "$GODOT" --headless --export-release "Web" ../build/web/index.html
fi
if [[ "$target" == server || "$target" == all ]]; then
  mkdir -p ../build/server
  "$GODOT" --headless --export-release "Linux Server" ../build/server/the-game-server.x86_64
fi
