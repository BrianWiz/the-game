# The Game

A multiplayer first-person sandbox with Source-style movement (strafe-jumping, timed
b-hops), built by friends and LLM agents through Discord. See
[docs/architecture.md](docs/architecture.md).

| Dir | Status |
|---|---|
| `game/` | Godot 4.7 client and dedicated server: room, Source movement, networking |
| `harness/` | `verify.sh` (agent definition of done); runner is planned |
| `bot/` | Go Discord bot (planned) |
| `api/` | Go accounts service (planned) |

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
godot --path game --headless -- --server --port=7777
godot --path game -- --connect=ws://127.0.0.1:7777   # run 2+ times
```

Web: `game/scripts/export.sh web` exports to `build/web/`. Serve it with
`python3 -m http.server -d build/web`, and add `?server=ws://127.0.0.1:7777` to the URL
to join a server.

## Controls

WASD to move, Space or the mouse wheel to jump (time the press on landing to b-hop),
mouse to look, Esc to release the cursor.
