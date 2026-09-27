class_name MovementConfig
extends Resource
## Source-engine movement tunables.
##
## Values are authored in Source "hammer units" (1 unit = 1 inch) so they can be
## compared directly against Source cvars (sv_accelerate, sv_airaccelerate, ...).
## Use the `*_m` helpers to get meters for Godot.

const UNIT_TO_METERS := 0.0254

## sv_maxspeed: max wish speed on the ground, u/s.
@export var max_speed := 320.0
## sv_accelerate: ground acceleration coefficient (dimensionless, per second).
@export var accelerate := 10.0
## sv_airaccelerate: air acceleration coefficient. 10 = classic, 100 = bhop/surf servers.
@export var air_accelerate := 100.0
## Cap on wish speed while airborne, u/s. This cap is what makes air-strafing work.
@export var air_wish_speed_cap := 30.0
## sv_friction: ground friction coefficient.
@export var friction := 4.0
## sv_stopspeed: below this speed friction acts as if moving at this speed, u/s.
@export var stop_speed := 100.0
## sv_gravity, u/s^2.
@export var gravity := 800.0
## Upward speed applied on jump, u/s. sqrt(2 * 800 * 57) = CS:S jump.
@export var jump_speed := 301.993377
## Moving up faster than this means you are not grounded (NON_JUMP_VELOCITY), u/s.
@export var non_jump_velocity := 140.0

@export_group("Hull")
## Player hull height, u (Source standing hull is 72).
@export var hull_height := 72.0
## Player hull width, u (Source hull is 32x32).
@export var hull_width := 32.0
## Eye height above the hull bottom, u.
@export var eye_height := 64.0


func max_speed_m() -> float:
	return max_speed * UNIT_TO_METERS


func air_wish_speed_cap_m() -> float:
	return air_wish_speed_cap * UNIT_TO_METERS


func stop_speed_m() -> float:
	return stop_speed * UNIT_TO_METERS


func gravity_m() -> float:
	return gravity * UNIT_TO_METERS


func jump_speed_m() -> float:
	return jump_speed * UNIT_TO_METERS


func non_jump_velocity_m() -> float:
	return non_jump_velocity * UNIT_TO_METERS


func hull_height_m() -> float:
	return hull_height * UNIT_TO_METERS


func hull_radius_m() -> float:
	return hull_width * 0.5 * UNIT_TO_METERS


func eye_height_m() -> float:
	return eye_height * UNIT_TO_METERS
