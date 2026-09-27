extends CanvasLayer
## Speed readout, connection status, and click-to-play pointer lock.

@onready var _speed: Label = $Speed
@onready var _status: Label = $Status
@onready var _overlay: Control = $Overlay


func _ready() -> void:
	Network.mode_changed.connect(_on_mode_changed)
	Network.connection_failed.connect(_on_connection_failed)
	_on_mode_changed(Network.mode)


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


func _process(_delta: float) -> void:
	_overlay.visible = (
		Input.mouse_mode != Input.MOUSE_MODE_CAPTURED
		and not get_tree().get_first_node_in_group(&"modal_ui")
	)
	var player := get_tree().get_first_node_in_group(&"local_player") as Player
	_speed.text = "%d u/s" % roundi(player.horizontal_speed_units()) if player else ""


func _on_mode_changed(mode: Network.Mode) -> void:
	var mode_name: String = Network.Mode.keys()[mode].to_lower()
	_status.text = "%s · %s" % [mode_name, Network.short_version(Network.build_version)]


func _on_connection_failed(reason: String) -> void:
	# Keep the corner short; the menu shows the full reason.
	var short := reason.get_slice(". ", 0).get_slice("; ", 0)
	_status.text = "offline · %s (%s)" % [Network.short_version(Network.build_version), short]
