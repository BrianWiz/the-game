extends CanvasLayer
## Keeps the web client on the canonical build: the one the server runs.
##
## `Network` already rejects a join whose build doesn't match the server's and reports
## the server's build in `server_version_mismatch` (core/net/network.gd). The login
## screen's silent reconnect (`ui/login/login_screen.gd`) retries every few seconds; this
## used to reload the page the instant any of those retries failed on a mismatch, which
## chained into a tight reload loop for as long as a deploy was in flight — the "refresh
## spamming" players saw. Now a mismatch shows a plain "Updating…" screen and reloads at
## most once every RELOAD_DELAY_S, so every reconnect attempt (the first join, or a
## dropped-connection retry) still doubles as a poll for a newer build, but the player
## sees a steady message instead of the page flashing.
##
## A plain reload keeps the page's URL and leaves localStorage alone, so the session
## token there (`core/net/account_api.gd`) survives and the returning player's client
## signs back in and rejoins on its own instead of losing their place.

const UI_THEME := preload("res://ui/theme/ui_theme.tres")
const MODAL_GROUP := &"modal_ui"
const PANEL_WIDTH := 360.0
## How long the "updating" screen stays up before reloading, so it reads instead of just
## flashing, and so reconnect retries don't chain reload after reload back to back.
const RELOAD_DELAY_S := 4.0

var _backdrop: Control
var _reload_timer: Timer


func _ready() -> void:
	layer = 20
	_build()
	if not OS.has_feature("web"):
		return
	_reload_timer = Timer.new()
	_reload_timer.one_shot = true
	_reload_timer.timeout.connect(_reload)
	add_child(_reload_timer)
	Network.connection_failed.connect(_on_connection_failed)


func _on_connection_failed(_reason: String) -> void:
	if not should_refresh(Network.server_version_mismatch):
		return
	show_updating_overlay()
	if _reload_timer.is_stopped():
		_reload_timer.start(RELOAD_DELAY_S)


func _reload() -> void:
	JavaScriptBridge.eval("window.location.reload()")


## Shows the full-screen "updating" panel and claims the modal group, so the idle-mouse
## menu (`ui/login/login_screen.gd`) doesn't pop open on top of it while a reload is due.
func show_updating_overlay() -> void:
	_backdrop.visible = true
	add_to_group(MODAL_GROUP)


## True once the server has told us our build is stale (a non-empty
## Network.server_version_mismatch; see core/net/network.gd).
static func should_refresh(server_version_mismatch: String) -> bool:
	return not server_version_mismatch.is_empty()


func _build() -> void:
	_backdrop = ColorRect.new()
	_backdrop.color = Color(0.05, 0.06, 0.08, 0.85)
	_backdrop.visible = false
	_backdrop.theme = UI_THEME
	_backdrop.set_anchors_preset(Control.PRESET_FULL_RECT)
	add_child(_backdrop)

	var center := CenterContainer.new()
	center.set_anchors_preset(Control.PRESET_FULL_RECT)
	_backdrop.add_child(center)

	var panel := PanelContainer.new()
	panel.custom_minimum_size.x = PANEL_WIDTH
	center.add_child(panel)

	var box := VBoxContainer.new()
	box.add_theme_constant_override("separation", 12)
	panel.add_child(box)

	var heading := Label.new()
	heading.text = "Updating…"
	heading.theme_type_variation = &"HeadingLabel"
	box.add_child(heading)

	var body := Label.new()
	body.text = "The game just shipped a new version. Hang tight, this reloads on its own."
	body.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	box.add_child(body)
