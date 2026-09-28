extends GutTest

const CoinsScene := preload("res://features/coins/feature.tscn")


func test_places_three_pickups_in_the_world() -> void:
	var coins := CoinsScene.instantiate() as Node3D
	add_child_autofree(coins)
	var pickups := 0
	for child: Node in coins.get_children():
		if child is CoinPickup:
			pickups += 1
	assert_eq(pickups, 3)
