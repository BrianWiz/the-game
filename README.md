# The Game

A multiplayer first-person sandbox with Source-style movement (strafe-jumping, timed
b-hops), built by friends and LLM agents through Discord. See
[docs/architecture.md](docs/architecture.md).

| Dir | Status |
|---|---|
| `game/` | Godot 4.7 client and dedicated server: room, Source movement, networking |
| `harness/` | Agent runner and `agent.yml`: label an issue `agent` to get a PR ([setup](harness/README.md)) |
| `bot/` | Go Discord bot (planned) |
| `api/` | Go accounts service: email/Discord sign-in, display names, join tickets |

## Quick start

Requires Godot 4.7.2 on `PATH` as `godot` (`brew install --cask godot`) and `uv` for the
GDScript linters.

```bash
godot --path game --editor                   # open in the editor
godot --path game                            # play offline
harness/verify.sh                            # lint + tests + multiplayer smoke
```

Local multiplayer:

```bash
godot --path game --headless -- --server --port=7777 --dev-insecure-auth
godot --path game -- --connect=ws://127.0.0.1:7777 --dev-insecure-auth --name=Alice
```

`--dev-insecure-auth` skips accounts. To use real ones, run the API locally:

```bash
openssl rand -hex 32 > /tmp/ticket-key
(cd api && API_ADDR=127.0.0.1:8080 API_DB=/tmp/api.db API_TICKET_KEY_FILE=/tmp/ticket-key \
  API_DEV_LOG_MAIL=true API_ALLOWED_ORIGINS=http://localhost:8060 \
  API_CLIENT_URL=http://localhost:8060/ go run ./cmd/api)
godot --path game --headless -- --server --port=7777 --ticket-key-file=/tmp/ticket-key
```

Web: `game/scripts/export.sh web` exports to `build/web/`. Serve it with
`python3 -m http.server -d build/web 8060`, and add
`?server=ws://127.0.0.1:7777&api=http://127.0.0.1:8080/api` to the URL to join a local
server. Sign-up emails show up in the API's log.

## Controls

WASD to move, Space or the mouse wheel to jump (time the press on landing to b-hop),
mouse to look, Esc for the menu (resume, display name, leave, sign out).
