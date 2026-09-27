extends CanvasLayer
## Corner readouts (version top left, players and connection top right) and
## click-to-play pointer lock. Styled with Kenney's UI Pack - Space Expansion.

const REFRESH_S := 0.25

var _refresh_in := 0.0

@onready var _release: Label = $Corners/Version/Line/Release
@onready var _commit: Label = $Corners/Version/Line/Commit
@onready var _count: Label = $Corners/Players/Lines/Count
@onready var _status: Label = $Corners/Players/Lines/Status
@onready var _overlay: Control = $Overlay


func _ready() -> void:
	_release.text = release_text(Network.game_version())
	# The hash sits in the body font: Kenney Future would uppercase it.
	_commit.text = Network.short_version(Network.build_version)


func _input(event: InputEvent) -> void:
	# Leave the mouse alone while a menu (e.g. the login screen) is open.
	if get_tree().get_first_node_in_group(&"modal_ui"):
		return
	# Browsers only grant pointer lock inside a user-gesture handler, so capture on click.
	var click := event as InputEventMouseButton
	if click and click.pressed and Input.mouse_mode != Input.MOUSE_MODE_CAPTURED:
		Input.mouse_mode = Input.MOUSE_MODE_CAPTURED
		get_viewport().set_input_as_handled()
	elif event.is_action_pressed("release_mouse"):
		Input.mouse_mode = Input.MOUSE_MODE_VISIBLE


func _process(delta: float) -> void:
	_overlay.visible = (
		Input.mouse_mode != Input.MOUSE_MODE_CAPTURED
		and not get_tree().get_first_node_in_group(&"modal_ui")
	)
	_refresh_in -= delta
	if _refresh_in <= 0.0:
		_refresh_in = REFRESH_S
		_count.text = player_count_text(_player_count())
		_status.text = _connection_text()


## "v0.3.0", shown next to the build's short commit hash.
static func release_text(game_version: String) -> String:
	return "v" + game_version


static func player_count_text(count: int) -> String:
	return "%d player%s" % [count, "" if count == 1 else "s"]


func _player_count() -> int:
	var count := 0
	for node: Node in get_tree().get_nodes_in_group(&"players"):
		if not node.is_queued_for_deletion():
			count += 1
	return count


func _connection_text() -> String:
	match Network.mode:
		Network.Mode.CLIENT:
			# The server shows up as a peer only once our join ticket is accepted.
			var joined := multiplayer.get_peers().has(MultiplayerPeer.TARGET_PEER_SERVER)
			return "online" if joined else "connecting…"
		Network.Mode.SERVER:
			return "server"
		_:
			return "offline"
