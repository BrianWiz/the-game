extends GutTest
## Pure movement math tests. Run: ./scripts/test.sh (from game/).

const DT := 1.0 / 64.0
const U := MovementConfig.UNIT_TO_METERS

var cfg: MovementConfig


func before_each() -> void:
	cfg = MovementConfig.new()


func _units(speed_m: float) -> float:
	return speed_m / U


func test_wish_direction_forward_is_negative_z_at_zero_yaw() -> void:
	var dir := SourceMovement.wish_direction(0.0, Vector2(0, -1))
	assert_almost_eq(dir, Vector3(0, 0, -1), Vector3.ONE * 0.0001)


func test_wish_direction_is_normalized_on_diagonal() -> void:
	var dir := SourceMovement.wish_direction(0.3, Vector2(1, -1).normalized())
	assert_almost_eq(dir.length(), 1.0, 0.0001)


func test_ground_movement_converges_to_max_speed() -> void:
	var vel := Vector3.ZERO
	var dir := Vector3(0, 0, -1)
	for i: int in 256:
		vel = SourceMovement.step(vel, dir, true, false, cfg, DT).velocity
	assert_almost_eq(_units(SourceMovement.horizontal_speed(vel)), cfg.max_speed, 1.0)


func test_friction_stops_player_without_input() -> void:
	var vel := Vector3(cfg.max_speed_m(), 0, 0)
	for i: int in 128:
		vel = SourceMovement.step(vel, Vector3.ZERO, true, false, cfg, DT).velocity
	assert_almost_eq(SourceMovement.horizontal_speed(vel), 0.0, 0.0001)


func test_jump_only_when_grounded() -> void:
	var grounded := SourceMovement.step(Vector3.ZERO, Vector3.ZERO, true, true, cfg, DT)
	assert_true(grounded.jumped)
	assert_gt(grounded.velocity.y, 0.0)
	var airborne := SourceMovement.step(Vector3.ZERO, Vector3.ZERO, false, true, cfg, DT)
	assert_false(airborne.jumped)


func test_perfect_hop_skips_friction() -> void:
	var vel := Vector3(cfg.max_speed_m() * 1.5, -1.0, 0)
	var result := SourceMovement.step(vel, Vector3.ZERO, true, true, cfg, DT)
	assert_true(result.jumped)
	assert_almost_eq(
		SourceMovement.horizontal_speed(result.velocity), cfg.max_speed_m() * 1.5, 0.0001
	)


func test_late_hop_loses_speed_to_friction() -> void:
	var vel := Vector3(cfg.max_speed_m() * 1.5, -1.0, 0)
	var landed := SourceMovement.step(vel, Vector3.ZERO, true, false, cfg, DT)
	assert_lt(SourceMovement.horizontal_speed(landed.velocity), cfg.max_speed_m() * 1.5)


func test_air_strafe_gains_speed_beyond_max() -> void:
	# Strafe perpendicular to velocity, turning with it, like a real air-strafe.
	var vel := Vector3(0, 0, -cfg.max_speed_m())
	for i: int in 64:
		var heading := Vector3(vel.x, 0, vel.z).normalized()
		var perpendicular := heading.cross(Vector3.UP)
		vel = SourceMovement.step(vel, perpendicular, false, false, cfg, DT).velocity
	assert_gt(_units(SourceMovement.horizontal_speed(vel)), cfg.max_speed + 50.0)


func test_air_acceleration_respects_wish_cap_when_holding_forward() -> void:
	# Holding forward in the air can't push projected speed past the 30 u/s cap.
	var vel := Vector3(0, 0, -cfg.max_speed_m())
	var result := SourceMovement.step(vel, Vector3(0, 0, -1), false, false, cfg, DT)
	assert_almost_eq(SourceMovement.horizontal_speed(result.velocity), cfg.max_speed_m(), 0.0001)


func test_upward_velocity_is_not_grounded() -> void:
	var fast_up := Vector3(0, cfg.non_jump_velocity_m() + 0.1, 0)
	assert_false(SourceMovement.is_grounded(true, fast_up, cfg))
