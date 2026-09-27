extends GutTest
## Join-ticket verification (core/net/join_ticket.gd) and the server's ticket check.
## VECTOR_* is shared with api/internal/ticket/ticket_test.go: the Go signer must produce
## exactly this ticket, so the two implementations can't drift apart silently.

const VECTOR_KEY := "0123456789abcdef0123456789abcdef"
const VECTOR_TICKET := (
	"v1.eyJhaWQiOjQyLCJuYW1lIjoiQm9iX3RoZV9CdWlsZGVyIiwiZXhwIjoyMDAwMDAwMDAwLCJub25jZSI6Im5v"
	+ "bmNlLTEyMyJ9.A7eknlwKGIu3Q+/X3P4pFog5M/yLwv/cQK4SQuKBzNE="
)
const VECTOR_EXP := 2000000000
const NOW := VECTOR_EXP - 60

var _saved_key: PackedByteArray
var _saved_insecure: bool


func before_each() -> void:
	_saved_key = Network.ticket_key
	_saved_insecure = Network.insecure_auth
	Network.ticket_key = VECTOR_KEY.to_utf8_buffer()
	Network.insecure_auth = false
	Network._used_nonces.clear()


func after_each() -> void:
	Network.ticket_key = _saved_key
	Network.insecure_auth = _saved_insecure
	Network._used_nonces.clear()


func _key() -> PackedByteArray:
	return VECTOR_KEY.to_utf8_buffer()


func test_valid_vector_verifies() -> void:
	var claims := JoinTicket.verify(VECTOR_TICKET, _key(), NOW)
	assert_eq(claims.get("account_id"), 42)
	assert_eq(claims.get("name"), "Bob_the_Builder")
	assert_eq(claims.get("expires"), VECTOR_EXP)
	assert_eq(claims.get("nonce"), "nonce-123")


func test_wrong_key_is_rejected() -> void:
	var other := "fedcba9876543210fedcba9876543210".to_utf8_buffer()
	assert_eq(JoinTicket.verify(VECTOR_TICKET, other, NOW), {})


func test_short_key_is_rejected() -> void:
	assert_eq(JoinTicket.verify(VECTOR_TICKET, "short".to_utf8_buffer(), NOW), {})


func test_tampered_payload_is_rejected() -> void:
	var tampered := VECTOR_TICKET.replace("eyJhaWQiOjQy", "eyJhaWQiOjQz")
	assert_ne(tampered, VECTOR_TICKET)
	assert_eq(JoinTicket.verify(tampered, _key(), NOW), {})


func test_expired_ticket_is_rejected() -> void:
	assert_eq(JoinTicket.verify(VECTOR_TICKET, _key(), VECTOR_EXP), {})


func test_overlong_lifetime_is_rejected() -> void:
	var early := VECTOR_EXP - JoinTicket.MAX_LIFETIME_S - 1
	assert_eq(JoinTicket.verify(VECTOR_TICKET, _key(), early), {})


func test_garbage_is_rejected_quietly() -> void:
	for bad: String in ["", "v1.", "v1.x.y", "v2.abc.def", "v1.!!!!.====", "dev:Bob"]:
		assert_eq(JoinTicket.verify(bad, _key(), NOW), {}, bad)


func test_server_rejects_replayed_nonce() -> void:
	var first := Network.authenticate_ticket(VECTOR_TICKET, 5, NOW)
	assert_eq(first, {"account_id": 42, "name": "Bob_the_Builder"})
	assert_eq(Network.authenticate_ticket(VECTOR_TICKET, 6, NOW), {})


func test_dev_tickets_need_insecure_mode() -> void:
	assert_eq(Network.authenticate_ticket("dev:Alice", 7, NOW), {})
	Network.insecure_auth = true
	assert_eq(Network.authenticate_ticket("dev:Alice", 7, NOW), {"account_id": 0, "name": "Alice"})
	var odd := Network.authenticate_ticket("dev:<b>x y</b>", 8, NOW)
	assert_eq(odd.get("name"), "bxyb", "dev names are reduced to [A-Za-z0-9_]")
	assert_eq(Network.authenticate_ticket("dev:", 9, NOW).get("name"), "dev9")


func test_load_key_trims_and_requires_length() -> void:
	var path := "user://test_ticket_key"
	var file := FileAccess.open(path, FileAccess.WRITE)
	file.store_string(VECTOR_KEY + "\n")
	file.close()
	assert_eq(JoinTicket.load_key(path), _key())
	file = FileAccess.open(path, FileAccess.WRITE)
	file.store_string("too short\n")
	file.close()
	assert_eq(JoinTicket.load_key(path), PackedByteArray())
	DirAccess.remove_absolute(ProjectSettings.globalize_path(path))
