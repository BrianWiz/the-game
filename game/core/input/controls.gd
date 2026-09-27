extends Node
## Registers default input actions at startup.
##
## Bindings are defined in code instead of project.godot so they're easy to read,
## diff, and merge. Features add their own actions with `ensure_action`.

const MOUSE_YAW_DEGREES := 0.022  ## Source m_yaw/m_pitch: degrees per mouse count.

## Source-style sensitivity (same number as the `sensitivity` cvar).
var sensitivity := 2.0


func _enter_tree() -> void:
	ensure_action("move_forward", [_key(KEY_W), _key(KEY_UP)])
	ensure_action("move_back", [_key(KEY_S), _key(KEY_DOWN)])
	ensure_action("move_left", [_key(KEY_A), _key(KEY_LEFT)])
	ensure_action("move_right", [_key(KEY_D), _key(KEY_RIGHT)])
	# Scroll-wheel jump is the classic b-hop bind: each notch is one press.
	ensure_action(
		"jump", [_key(KEY_SPACE), _mouse(MOUSE_BUTTON_WHEEL_DOWN), _mouse(MOUSE_BUTTON_WHEEL_UP)]
	)
	ensure_action("release_mouse", [_key(KEY_ESCAPE)])


## Radians of rotation per mouse count at the current sensitivity.
func look_radians_per_count() -> float:
	return deg_to_rad(MOUSE_YAW_DEGREES * sensitivity)


func ensure_action(action: StringName, events: Array[InputEvent]) -> void:
	if not InputMap.has_action(action):
		InputMap.add_action(action)
	for event: InputEvent in events:
		if not InputMap.action_has_event(action, event):
			InputMap.action_add_event(action, event)


func _key(keycode: Key) -> InputEventKey:
	var event := InputEventKey.new()
	event.physical_keycode = keycode
	return event


func _mouse(button: MouseButton) -> InputEventMouseButton:
	var event := InputEventMouseButton.new()
	event.button_index = button
	return event
