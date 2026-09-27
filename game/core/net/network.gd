extends Node
## Owns the MultiplayerPeer and decides how this process runs.
##
## Authority model:
## - Server (peer 1) is authoritative for everything: spawning, despawning,
##   spawn positions, and all game state added by features.
## - The one exception is player movement: each client is the multiplayer
##   authority of its own Player node, simulates Source movement locally, and
##   replicates it via MultiplayerSynchronizer.
##
## Modes (picked in `start_from_environment`):
##   dedicated server: `godot --headless -- --server [--port=7777]`
##   client:           `-- --connect=ws://host:7777`, or `?server=wss://...` on web
##   offline:          `-- --offline`, `?server=offline`, or no server configured;
##                     OfflineMultiplayerPeer, this process is the server
##
## Web builds with no `?server=` join `game/network/default_server_url`.
## Native builds default to offline so local development needs no backend.

signal mode_changed(mode: Mode)
signal connection_failed(reason: String)

enum Mode { OFFLINE, SERVER, CLIENT }

const DEFAULT_PORT := 7777
const DEFAULT_SERVER_SETTING := "game/network/default_server_url"

var mode := Mode.OFFLINE
## User args after `--`, e.g. {"server": "", "port": "7777"}.
var args := {}


func _enter_tree() -> void:
	args = _parse_user_args()


## Whether a flag like `--debug-roster` was passed after `--`.
func has_flag(flag: String) -> bool:
	return args.has(flag)


## Pick a mode from command-line user args (after `--`) or the web page URL.
func start_from_environment() -> void:
	if args.has("server"):
		start_server(int(args.get("port", str(DEFAULT_PORT))))
		return
	var url := resolve_server_url()
	if url.is_empty():
		start_offline()
	else:
		join(url)


## Server URL to join, or "" for offline. Precedence: --offline, --connect=,
## ?server= (web), then the project default (web only).
func resolve_server_url() -> String:
	if args.has("offline"):
		return ""
	var url: String = args.get("connect", "")
	if url.is_empty() and OS.has_feature("web"):
		url = _web_query_param("server")
		if url.is_empty():
			url = str(ProjectSettings.get_setting(DEFAULT_SERVER_SETTING, ""))
	return "" if url == "offline" else url


func start_server(port: int) -> Error:
	var peer := WebSocketMultiplayerPeer.new()
	var err := peer.create_server(port)
	if err != OK:
		push_error("Failed to listen on port %d: %s" % [port, error_string(err)])
		return err
	multiplayer.multiplayer_peer = peer
	print("Server listening on port %d" % port)
	_set_mode(Mode.SERVER)
	return OK


func join(url: String) -> Error:
	var peer := WebSocketMultiplayerPeer.new()
	var err := peer.create_client(url)
	if err != OK:
		connection_failed.emit(error_string(err))
		return err
	multiplayer.multiplayer_peer = peer
	if not multiplayer.connection_failed.is_connected(_on_connection_failed):
		multiplayer.connection_failed.connect(_on_connection_failed)
		multiplayer.server_disconnected.connect(_on_server_disconnected)
	print("Connecting to %s" % url)
	_set_mode(Mode.CLIENT)
	return OK


func start_offline() -> void:
	multiplayer.multiplayer_peer = OfflineMultiplayerPeer.new()
	_set_mode(Mode.OFFLINE)


## True when this process runs server logic (dedicated server or offline).
func is_authoritative() -> bool:
	return multiplayer.is_server()


func _set_mode(new_mode: Mode) -> void:
	mode = new_mode
	mode_changed.emit(mode)


func _on_connection_failed() -> void:
	connection_failed.emit("Could not connect to server")
	start_offline()


func _on_server_disconnected() -> void:
	connection_failed.emit("Disconnected from server")
	start_offline()


func _parse_user_args() -> Dictionary:
	var result := {}
	for arg: String in OS.get_cmdline_user_args():
		var trimmed := arg.trim_prefix("--")
		var parts := trimmed.split("=", true, 1)
		result[parts[0]] = parts[1] if parts.size() > 1 else ""
	return result


func _web_query_param(key: String) -> String:
	if not OS.has_feature("web"):
		return ""
	var value: Variant = JavaScriptBridge.eval(
		"new URLSearchParams(window.location.search).get('%s') || ''" % key.c_escape()
	)
	return str(value) if value != null else ""
