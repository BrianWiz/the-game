# bot

The Discord bot. Friends ask for features with `/feature`; the bot opens a GitHub issue
through its GitHub App, starts `agent.yml`, and reports progress in a Discord thread.
`/revise` in that thread asks the agent to change the PR.

| Path | What |
|---|---|
| `cmd/bot` | Configuration, wiring, HTTP server, `bot healthcheck` |
| `internal/core` | Commands, limits, GitHub event handling, reconcile (no Discord or HTTP) |
| `internal/discordbot` | disgo gateway client: registers the guild commands, posts to threads |
| `internal/github` | GitHub App auth (JWT, installation token), the REST calls used, webhook checks |
| `internal/webhook` | `POST /bot/github`: signature, deduplication, one-at-a-time processing |
| `internal/store` | SQLite (`/data/bot.db`): jobs, runs, handled deliveries and comments |

## How it works

1. **`/feature <request>`** in a text channel (optionally only `BOT_FEATURE_CHANNEL_ID`).
   The user needs `BOT_REQUESTER_ROLE_ID`. The bot reserves a run against the limits,
   opens an issue whose body ends in `Requested-by: <name> <discord:<id>>` (the harness
   credits that person in the PR), answers publicly, opens a thread on the answer, and
   dispatches `agent.yml` (`mode=implement`, `request_id=bot-<run>`).
2. **Progress** reaches the thread from the App's webhooks:
   - `issue_comment`: the harness's 🤖 comments on the issue or PR ("Starting…",
     "Opened <PR>", failures with a link to the logs).
   - `workflow_run`: agent runs are matched by their run name
     (`agent #N mode [bot-<run>]`) to track status. The bot speaks up itself only if a
     run ends without a result, such as when it's cancelled. `game-ci` runs on
     `agent/<issue>-…` branches post "CI passed" or "CI failed".
   - `pull_request`: merged or closed.
   A reconcile loop (every 2 minutes, only while runs are active) polls the agent runs and
   comments in case a webhook was missed, and expires runs that never started.
3. **`/revise <changes>`** inside a feature thread, once its PR exists, from the requester
   or anyone with the role, and only when no run is active for it. It dispatches
   `mode=revise` on the PR, with the text as the newest instructions
   ("From <name> on Discord: …").

**Limits** count every run (features and revisions) except ones that never started:
`BOT_RUNS_PER_USER` per rolling 24 hours (default 5), and at most `BOT_MAX_ACTIVE_RUNS`
at once (default 2). Active runs older than 3 hours stop counting.

Messages never ping anyone except the requester, and only on their own job's results.
Requests are copied into issues with `@` defused, so they can't ping GitHub users.

## Configuration

Environment variables; secrets are files.

| Variable | Default | |
|---|---|---|
| `BOT_ADDR` | `:8081` | HTTP listen address (`/bot/github`, `/bot/health`) |
| `BOT_DB` | `/data/bot.db` | SQLite database |
| `BOT_REPO` | `tfpp/the-game` | Repository the App is installed on |
| `BOT_GITHUB_CLIENT_ID` | required | GitHub App client ID |
| `BOT_GITHUB_PRIVATE_KEY_FILE` | `/run/secrets/bot/github-app.pem` | App private key |
| `BOT_GITHUB_WEBHOOK_SECRET_FILE` | `/run/secrets/bot/github-webhook-secret` | Webhook secret |
| `BOT_DISCORD_TOKEN_FILE` | `/run/secrets/bot/discord-token` | Discord bot token |
| `BOT_GUILD_ID` | required | The Discord server |
| `BOT_REQUESTER_ROLE_ID` | required | Role allowed to use `/feature` and `/revise` |
| `BOT_FEATURE_CHANNEL_ID` | any channel | Only channel `/feature` works in |
| `BOT_RUNS_PER_USER` | `5` | Runs per user per 24 hours |
| `BOT_MAX_ACTIVE_RUNS` | `2` | Concurrent runs |
| `BOT_REF`, `BOT_WORKFLOW`, `BOT_CI_WORKFLOW`, `BOT_AGENT` | `main`, `agent.yml`, `game-ci.yml`, `claude` | |

## Setup

### GitHub App

Create it under the `tfpp` organization (Settings → Developer settings → GitHub Apps):

- **Webhook:** active, URL `https://game.chrisbox.dev/bot/github`, secret from
  `openssl rand -hex 32`.
- **Repository permissions:** Actions read and write (dispatch, and `workflow_run`
  events); Contents, Issues and Pull requests read and write; Metadata read. Leave
  **Workflows at no access**, so agents can't change CI.
- **Events:** Issue comment, Pull request, Workflow run.
- **Installable:** only on this account. Install it on `tfpp/the-game` only.
- Generate a private key and note the **client ID**.

Then let `agent.yml` publish as the App and trust its dispatches:

```bash
gh variable set AGENT_APP_CLIENT_ID --body <client id>
gh secret set AGENT_APP_PRIVATE_KEY < <app>.private-key.pem
gh variable set AGENT_GIT_NAME --body '<slug>[bot]'
gh variable set AGENT_GIT_EMAIL --body "$(gh api '/users/<slug>[bot]' --jq .id)+<slug>[bot]@users.noreply.github.com"
gh variable set AGENT_TRUSTED_BOTS --body '<slug>[bot]'
```

### Discord application

In the [developer portal](https://discord.com/developers/applications):

- **Bot:** reset and copy the token. No privileged intents. Turn off "Public Bot".
- **Installation:** guild install with scopes `bot` and `applications.commands`, and the
  permissions View Channels, Send Messages, Create Public Threads and Send Messages in
  Threads. Open the install link and add the bot to the server.
- With Developer Mode on (User Settings → Advanced), copy the server ID, the requester
  role's ID and, optionally, the feature channel's ID.

The bot registers `/feature` and `/revise` in that server when it starts.

### Deploy

The image is `ghcr.io/tfpp/the-game-bot`, built by `bot-image.yml` on pushes to `main`
that touch `bot/`. It runs as UID 10040 with a read-only root filesystem. State goes in
`/data`, and the three secret files in `/run/secrets/bot/`. Add a Cloudflare Tunnel route
for exactly `game.chrisbox.dev` path `^/bot/github$` → `http://bot:8081`, placed before
the catch-all game-server rule. Don't route `/bot/health` publicly.

## Local run

```bash
cd bot
BOT_DB=/tmp/bot.db BOT_GITHUB_CLIENT_ID=... BOT_GUILD_ID=... BOT_REQUESTER_ROLE_ID=... \
  BOT_DISCORD_TOKEN_FILE=/tmp/discord-token BOT_GITHUB_PRIVATE_KEY_FILE=/tmp/app.pem \
  BOT_GITHUB_WEBHOOK_SECRET_FILE=/tmp/webhook-secret go run ./cmd/bot
```

Commands work locally (the gateway is outbound). Webhooks don't reach a laptop, but the
reconcile loop still relays agent comments and run status every 2 minutes, just not CI
results or merges.
