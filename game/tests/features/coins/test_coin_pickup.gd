extends GutTest

const PickupScene := preload("res://features/coins/pickup.tscn")
const PlayerScene := preload("res://core/player/player.tscn")

var _wallet: PlayerMoney
var _pickup: CoinPickup


func before_each() -> void:
	_wallet = PlayerMoney.new()
	add_child_autofree(_wallet)
	_wallet.set_process(false)
	_pickup = PickupScene.instantiate() as CoinPickup
	add_child_autofree(_pickup)
	_pickup.set_process(false)


func _player_at(position: Vector3, authority: int = 1) -> Player:
	var player := PlayerScene.instantiate() as Player
	player.set_multiplayer_authority(authority)
	player.position = position
	player.net_position = position
	add_child_autofree(player)
	return player


func test_nearby_player_can_use_an_available_pickup() -> void:
	var player := _player_at(Vector3.ZERO)
	assert_true(_pickup.can_use(player))


func test_far_player_cannot_use_the_pickup() -> void:
	var player := _player_at(Vector3(10, 0, 0))
	assert_false(_pickup.can_use(player))


func test_unavailable_pickup_cannot_be_used() -> void:
	var player := _player_at(Vector3.ZERO)
	_pickup.available = false
	assert_false(_pickup.can_use(player))


func test_collecting_adds_ten_dollars_and_disables_the_pickup() -> void:
	_player_at(Vector3.ZERO)
	await _pickup.request_collect()
	assert_false(_pickup.available)
	assert_eq(int(_wallet.balances[1]), 3000)


func test_unknown_player_cannot_collect() -> void:
	await _pickup.request_collect()
	assert_true(_pickup.available)
	assert_false(_wallet.balances.has(1))


func test_second_collect_is_ignored_until_cooldown() -> void:
	_player_at(Vector3.ZERO)
	await _pickup.request_collect()
	var first_balance := int(_wallet.balances[1])
	await _pickup.request_collect()
	assert_eq(int(_wallet.balances[1]), first_balance)


func test_cooldown_reopens_the_pickup() -> void:
	_player_at(Vector3.ZERO)
	await _pickup.request_collect()
	assert_false(_pickup.available)
	_pickup._on_cooldown_finished()
	assert_true(_pickup.available)
