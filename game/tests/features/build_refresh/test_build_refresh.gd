extends GutTest
## Gating logic and the "updating" overlay for build_refresh
## (features/build_refresh/build_refresh.gd). JavaScriptBridge-dependent behavior (the
## actual reload) isn't exercised here, since GUT runs headless, not on web.

const BuildRefresh := preload("res://features/build_refresh/build_refresh.gd")


func test_no_refresh_when_build_matches() -> void:
	assert_false(BuildRefresh.should_refresh(""))


func test_refreshes_when_server_reports_a_different_build() -> void:
	assert_true(BuildRefresh.should_refresh("abc1234"))


func test_show_updating_overlay_displays_it_and_claims_the_modal_group() -> void:
	var node := BuildRefresh.new()
	add_child_autofree(node)
	node.show_updating_overlay()
	assert_true(node.is_in_group(&"modal_ui"))
