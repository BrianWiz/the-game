class_name JoinTicket
extends RefCounted
## Verifies join tickets issued by the accounts API (`api/internal/ticket`).
##
## Format: "v1." + base64(payload) + "." + base64(HMAC-SHA256(key, "v1." + base64(payload)))
## with standard padded base64 and a JSON payload {"aid", "name", "exp", "nonce"}.
## The key is the trimmed contents of the shared key file, as bytes. Keep this in sync
## with the Go signer; tests/unit/test_join_ticket.gd shares a test vector with it.

const PREFIX := "v1."
const MIN_KEY_BYTES := 32
const MAX_TICKET_LENGTH := 1024
## Reject tickets that claim to stay valid longer than this (the API issues 60 s ones).
const MAX_LIFETIME_S := 300

static var _base64 := RegEx.create_from_string(
	"^(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$"
)


## Returns {"account_id": int, "name": String, "expires": int, "nonce": String}, or {}
## if the ticket is malformed, forged, expired or too long-lived. Doesn't check replay.
static func verify(ticket: String, key: PackedByteArray, now: int) -> Dictionary:
	if key.size() < MIN_KEY_BYTES or ticket.length() > MAX_TICKET_LENGTH:
		return {}
	if not ticket.begins_with(PREFIX):
		return {}
	var dot := ticket.rfind(".")
	if dot <= PREFIX.length():
		return {}
	var message := ticket.substr(0, dot)
	var encoded_payload := message.substr(PREFIX.length())
	var encoded_sig := ticket.substr(dot + 1)
	# Validate before decoding: Marshalls logs engine errors on bad base64.
	if not _is_base64(encoded_payload) or not _is_base64(encoded_sig):
		return {}
	var expected := Crypto.new().hmac_digest(
		HashingContext.HASH_SHA256, key, message.to_utf8_buffer()
	)
	var sig := Marshalls.base64_to_raw(encoded_sig)
	if sig.size() != expected.size() or not Crypto.new().constant_time_compare(expected, sig):
		return {}
	var parsed: Variant = JSON.parse_string(
		Marshalls.base64_to_raw(encoded_payload).get_string_from_utf8()
	)
	if not parsed is Dictionary:
		return {}
	var claims := parsed as Dictionary
	var aid: Variant = claims.get("aid")
	var display_name: Variant = claims.get("name")
	var exp: Variant = claims.get("exp")
	var nonce: Variant = claims.get("nonce")
	if not (aid is float or aid is int) or not (exp is float or exp is int):
		return {}
	if not display_name is String or not nonce is String or str(nonce).is_empty():
		return {}
	var expires := int(exp)
	if expires <= now or expires > now + MAX_LIFETIME_S:
		return {}
	return {"account_id": int(aid), "name": display_name, "expires": expires, "nonce": nonce}


## Reads a key file: its trimmed contents as bytes. Returns an empty array on failure.
static func load_key(path: String) -> PackedByteArray:
	if not FileAccess.file_exists(path):
		return PackedByteArray()
	var key := FileAccess.get_file_as_string(path).strip_edges().to_utf8_buffer()
	return key if key.size() >= MIN_KEY_BYTES else PackedByteArray()


static func _is_base64(text: String) -> bool:
	return not text.is_empty() and _base64.search(text) != null
