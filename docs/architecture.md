# Architecture

Friends post a feature request in Discord and an LLM coding agent builds it as a PR
against a multiplayer Godot sandbox. Trusted users approve merges from Discord, and the
game ships to GitHub Pages (client) and the homelab (server).

```
Discord ◀──▶ bot (Go, homelab)  ──GitHub App──▶  issues / PRs / workflow_dispatch
                 │                                        │
                 │ merge coordinator (serial)             ▼
                 │                          Actions: agent.yml → harness/ → PR
                 │                                   game-ci.yml → harness/verify.sh
                 ▼                                   pages.yml   → GitHub Pages (web client)
            api (Go, homelab)                        server-image.yml → ghcr.io → homelab
            accounts, sessions, join tickets
                 ▲
Browser ─────────┘  wss://game.chrisbox.dev (NPM) ──▶ game server (Godot headless, homelab)
```

## Repo layout

| Path | What | Language |
|---|---|---|
| `game/` | Godot 4.7 project: web client and dedicated server from one codebase | GDScript |
| `bot/` | Discord bot, job tracking, merge coordinator | Go |
| `api/` | Accounts (Discord OAuth plus email/password), sessions, game join tickets | Go |
| `harness/` | Agent runner: adapters, prompts, `verify.sh` (the definition of done) | bash |
| `.github/workflows/` | CI, Pages, server image, and later `agent.yml` | YAML |
| `docs/` | This file and ADR-style notes | Markdown |

`bot/` and `api/` are separate Go modules joined by a root `go.work`.

## Game (`game/`)

- **Engine:** Godot 4.7, GDScript, Jolt physics, and the Compatibility renderer (WebGL2).
  There is no compile step, so each agent iteration takes seconds rather than minutes.
- **Tick:** 64 Hz fixed physics (`physics/common/physics_ticks_per_second`) with physics
  interpolation turned on for rendering.
- **Movement:** Source `CGameMovement` math is a pure function
  (`core/movement/source_movement.gd`), and collision is Godot's `move_and_slide` on Jolt.
  Tunables in `MovementConfig` use Source units (320 max speed, 100 airaccelerate, 30 u/s
  air wish cap, 800 gravity). There is no auto-hop: a jump happens only on the tick the key
  is *pressed*, and pressing on the landing tick skips friction.
- **Web:** single-threaded web export, so no COOP/COEP headers are needed on GitHub Pages.
  The mouse is captured on click, because browsers require a user gesture for pointer lock.

### Networking and authority

Godot's high-level multiplayer runs over `WebSocketMultiplayerPeer`. WebSocket works in
browsers and passes through nginx-proxy-manager with TLS, so no UDP port-forward is needed.

| State | Authority | Mechanism |
|---|---|---|
| Player movement (position, velocity, view angles) | **Owning client** | `MultiplayerSynchronizer` on each `Player`; `set_multiplayer_authority(peer_id)` |
| Spawning and despawning players | Server | `MultiplayerSpawner` (`main.tscn/PlayerSpawner`) with a custom `spawn_function` |
| Respawns and teleports | Server requests it, owner applies it | `Player.server_teleport` RPC (rejected unless the sender is peer 1) |
| Everything else (features, world state, scores, items) | **Server** | Server-owned nodes, `MultiplayerSpawner` / `MultiplayerSynchronizer`, and client→server request RPCs |

Rules for features, repeated in `game/AGENTS.md`:
- Mutate shared state only when `multiplayer.is_server()` is true.
- Clients ask for changes with `@rpc("any_peer")` request RPCs, and the server validates
  the sender with `multiplayer.get_remote_sender_id()`.
- Replicate server state with a `MultiplayerSynchronizer` (authority 1) or a spawner.

Movement is client-authoritative, so remote players see what the owner simulated. There is
no server reconciliation. The server may add sanity checks (speed/teleport limits) and
respawn via RPC. That tradeoff is deliberate for a friends-only sandbox.

**Run modes** (`core/net/network.gd`):
- Dedicated server: `godot --headless -- --server --port=7777`.
- Client: `-- --connect=ws://host:7777`, or `?server=wss://…` on the web.
- Offline: the default. The process is its own server, so single player works without
  a backend.

## Accounts (`api/`, planned)

- **Sign-in:** either Discord OAuth2 or email and password (argon2id hashes, email
  verification, reset tokens). Both resolve to one `accounts` row, and a Discord identity
  can be linked to an email account.
- **Session:** the API returns a session token to the web client.
- **Joining a game:**
  1. The client calls `POST /join-ticket` and gets a short-lived ticket
     (HMAC/Ed25519-signed: account ID, display name, expiry).
  2. The client sends the ticket in its first RPC after connecting.
  3. The server verifies the ticket, then spawns the player. Unauthenticated peers are
     kicked after a timeout.
- **Storage:** SQLite on the homelab, shared with the bot so Discord users map to game
  accounts.

## Agent pipeline (planned)

1. `/feature <text>` in Discord. The bot checks the allowlist, rate limit and concurrency
   cap, opens a thread, and creates a GitHub issue through the GitHub App.
2. The bot dispatches `agent.yml`, which only accepts dispatches from the bot App.
   The workflow runs `harness/run.sh --agent claude|codex|pi --mode implement|revise|resolve-conflicts`.
3. The agent works on `agent/<issue>-<slug>`, must make `harness/verify.sh` pass, and opens
   a PR using an App token (so CI triggers).
4. The Action reports back to the thread with the PR link, CI status, and a preview link.
5. Replies in the thread become PR comments and start `revise` runs.

**Agents:** Claude Code runs on GitHub-hosted runners using `CLAUDE_CODE_OAUTH_TOKEN`
(from `claude setup-token`). Codex and pi need persisted `auth.json` logins, so they run on
a self-hosted, ephemeral runner on the homelab.

## Merging

Approvals come from Discord, and a single coordinator applies them in order.
- A trusted Discord role clicks **Approve & merge**. The approval is pinned to the PR's
  head SHA.
- The coordinator takes one PR at a time:
  1. Re-check the head SHA.
  2. If the branch is behind main, update it and wait for checks.
  3. Squash-merge with an expected `sha`.
- **Conflicts:** the agent runs in `resolve-conflicts` mode, and the new SHA needs
  re-approval.
- **After every merge:** the coordinator re-checks the other open PRs and warns their
  threads early about new conflicts.
- **Human-only paths:** PRs touching paths listed in `CODEOWNERS` (`.github/`, `harness/`,
  `bot/`, `api/`, core movement/net, `project.godot`) cannot be approved from Discord.

**Conflict avoidance:** each feature lives in `game/features/<name>/` and self-registers
(see `game/AGENTS.md`), so parallel PRs rarely touch the same files.

## Deploy

| Artifact | Built by | Runs on |
|---|---|---|
| Web client | `pages.yml` (Godot web export) | GitHub Pages |
| Dedicated server | `server-image.yml` → `ghcr.io/tfpp/the-game-server` | Homelab VM (docker compose, deployed from `~/code/homelab`) |
| bot, api | Their own images (planned) | Homelab VM |

The client and server must run the same code. The plan is a protocol/version check on
join, plus a server deploy triggered by the same merge that updates Pages.

## Milestones

1. **v0.1:** empty room, Source movement, first-person view, Pages deploy, dedicated
   server, CI. *(done in this scaffold)*
2. **v0.2:** deploy the server to the homelab behind NPM, and have the web client default
   to `wss://game.chrisbox.dev`.
3. **v0.3:** `api/` accounts (Discord plus email/password) and join tickets.
4. **v0.4:** `harness/` plus `agent.yml` (Claude), triggered by label/dispatch.
5. **v0.5:** `bot/` MVP (`/feature`, threads, status), then revise loops.
6. **v0.6:** Discord approvals plus the merge coordinator; later, a homelab runner with
   Codex/pi.
