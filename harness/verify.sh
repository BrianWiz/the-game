#!/usr/bin/env bash
# The agent's definition of done. Every change must pass this before a PR is opened.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"

"$root/game/scripts/check.sh"

for mod in bot api; do
  if [[ -f "$root/$mod/go.mod" ]]; then
    printf '\n==> go checks: %s\n' "$mod"
    (cd "$root/$mod" && gofmt -l . | (! grep .) && go vet ./... && go test ./...)
  fi
done
