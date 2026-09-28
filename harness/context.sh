#!/usr/bin/env bash
# Writes the task description for harness/run.sh from GitHub, on stdout.
# Usage: harness/context.sh --issue N [--pr N] [--instructions-file FILE]
# Needs GH_TOKEN (read access to issues and pull requests) and GITHUB_REPOSITORY or a
# gh default repo. Comments from bots (the harness itself) are left out.
set -euo pipefail
# shellcheck source=lib.sh source-path=SCRIPTDIR
source "$(dirname "$0")/lib.sh"

issue="" pr="" instructions=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --issue) issue="$2" ;;
    --pr) pr="$2" ;;
    --instructions-file) instructions="$2" ;;
    *) die "unknown argument: $1" ;;
  esac
  shift 2
done
[[ "$issue" =~ ^[0-9]+$ ]] || die "--issue N is required"
repo="${GITHUB_REPOSITORY:-$(gh repo view --json nameWithOwner --jq .nameWithOwner)}"

humans='map(select(.user.type != "Bot"))'

gh api "repos/$repo/issues/$issue" | jq -r "$ISSUE_JQ"'
  "## Request (issue #\(.number))\n\n**\(.title)**\n\n\(request_body)\n\n_Requested by \(requester)._\n"'

comments="$(gh api --paginate "repos/$repo/issues/$issue/comments" |
  jq -rs "add // [] | $humans | map(\"**@\(.user.login)**: \(.body | gsub(\"\r\"; \"\"))\") | join(\"\n\n\")")"
[[ -n "$comments" ]] && printf '\n## Discussion on the issue\n\n%s\n' "$comments"

if [[ -n "$pr" ]]; then
  [[ "$pr" =~ ^[0-9]+$ ]] || die "--pr must be a number"
  reviews="$(gh api --paginate "repos/$repo/pulls/$pr/reviews" |
    jq -rs "add // [] | $humans | map(select(.body != \"\" and .body != null)) |
      map(\"**@\(.user.login)** (review, \(.state | ascii_downcase)): \(.body | gsub(\"\r\"; \"\"))\") | join(\"\n\n\")")"
  inline="$(gh api --paginate "repos/$repo/pulls/$pr/comments" |
    jq -rs "add // [] | $humans |
      map(\"**@\(.user.login)** on \`\(.path):\(.line // .original_line // \"?\")\`: \(.body | gsub(\"\r\"; \"\"))\") | join(\"\n\n\")")"
  conversation="$(gh api --paginate "repos/$repo/issues/$pr/comments" |
    jq -rs "add // [] | $humans | map(\"**@\(.user.login)**: \(.body | gsub(\"\r\"; \"\"))\") | join(\"\n\n\")")"
  printf '\n## Feedback on pull request #%s (oldest first)\n' "$pr"
  for section in "$reviews" "$inline" "$conversation"; do
    [[ -n "$section" ]] && printf '\n%s\n' "$section"
  done
  [[ -z "$reviews$inline$conversation" ]] && printf '\n_No written feedback yet._\n'
fi

if [[ -n "$instructions" && -s "$instructions" ]]; then
  printf '\n## Latest instructions\n\n%s\n' "$(cat "$instructions")"
fi
exit 0
