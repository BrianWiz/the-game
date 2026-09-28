#!/usr/bin/env bash
# The agent's definition of done. Every change must pass this before a PR is opened.
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"

printf '==> harness checks\n'
for f in "$root"/harness/*.sh "$root"/harness/adapters/*.sh "$root"/harness/tests/*.sh; do
  bash -n "$f"
done
if command -v shellcheck >/dev/null; then
  shellcheck -x -S warning "$root"/harness/*.sh "$root"/harness/adapters/*.sh
fi
for t in "$root"/harness/tests/test_*.sh; do "$t"; done

"$root/game/scripts/check.sh"

for mod in bot api; do
  if [[ -f "$root/$mod/go.mod" ]]; then
    printf '\n==> go checks: %s\n' "$mod"
    (cd "$root/$mod" && gofmt -l . | (! grep .) && go vet ./... && go test ./...)
  fi
done
