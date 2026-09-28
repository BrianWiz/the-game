# Shared helpers for the harness scripts. Source it; don't run it.
# shellcheck shell=bash

HARNESS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$HARNESS_DIR/.." && pwd)"
export HARNESS_DIR REPO_ROOT

# Conventional Commits types accepted in PR titles and commit subjects.
CC_TITLE_RE='^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([a-z0-9._/-]+\))?!?: [^ ].{0,70}$'

log() { printf '[harness] %s\n' "$*" >&2; }
die() {
  log "error: $*"
  exit 1
}

# slugify "Add a Jump Pad!" -> add-a-jump-pad (max 40 chars, no trailing dash)
slugify() {
  local s
  s="$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | LC_ALL=C sed -E 's/[^a-z0-9]+/-/g; s/^-+//')"
  s="${s:0:40}"
  while [[ "$s" == *- ]]; do s="${s%-}"; done
  printf '%s' "${s:-feature}"
}

# branch_for_issue 12 "Add a jump pad" -> agent/12-add-a-jump-pad
branch_for_issue() { printf 'agent/%s-%s' "$1" "$(slugify "$2")"; }

# issue_from_branch agent/12-add-a-jump-pad -> 12 (fails for non-agent branches)
issue_from_branch() {
  [[ "$1" =~ ^agent/([0-9]+)- ]] || return 1
  printf '%s' "${BASH_REMATCH[1]}"
}

# Human-review paths, read from CODEOWNERS so there is one list.
protected_patterns() {
  local line
  while read -r line _; do
    [[ -z "$line" || "$line" == \#* ]] && continue
    printf '%s\n' "${line#/}"
  done <"$REPO_ROOT/.github/CODEOWNERS"
}

# Reads file paths on stdin and prints the ones under a protected pattern.
filter_protected() {
  local patterns f p
  patterns="$(protected_patterns)"
  while IFS= read -r f; do
    for p in $patterns; do
      if [[ "$p" == */ && "$f" == "$p"* ]] || [[ "$f" == "$p" || "$f" == "$p"/* ]]; then
        printf '%s\n' "$f"
        break
      fi
    done
  done
}

# valid_title "feat(game): add jump pads" -> exit 0
valid_title() { [[ "$1" =~ $CC_TITLE_RE ]]; }
