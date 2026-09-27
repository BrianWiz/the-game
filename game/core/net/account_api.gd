class_name AccountApi
extends Node
## Client for the accounts API (`api/`): sign-in, display names and join tickets.
##
## The session token is a bearer token kept in localStorage on the web (the API is on
## another site, so no cookies) and in user:// on native builds. Every call resolves to
## {"ok": bool, "status": int, "data": Dictionary, "error": String, "message": String}.

const SESSION_KEY := "the-game.session"
const VERIFIER_KEY := "the-game.discord-verifier"
const NATIVE_STORE := "user://account.cfg"
const TIMEOUT_S := 15.0

var base_url := ""
var session_token := ""


func _init(api_url: String = "") -> void:
	base_url = api_url.trim_suffix("/")
	session_token = load_value(SESSION_KEY)


func has_session() -> bool:
	return not session_token.is_empty()


func set_session(token: String) -> void:
	session_token = token
	save_value(SESSION_KEY, token)


func clear_session() -> void:
	session_token = ""
	save_value(SESSION_KEY, "")


## Sends a JSON request. `body` is serialized when not null.
func call_api(method: HTTPClient.Method, path: String, body: Variant = null) -> Dictionary:
	if base_url.is_empty():
		return _result(0, {}, "no_api", "No accounts server is configured")
	var http := HTTPRequest.new()
	http.timeout = TIMEOUT_S
	add_child(http)
	var headers := PackedStringArray(["Content-Type: application/json", "Accept: application/json"])
	if has_session():
		headers.append("Authorization: Bearer " + session_token)
	var payload := "" if body == null else JSON.stringify(body)
	var err := http.request(base_url + path, headers, method, payload)
	if err != OK:
		http.queue_free()
		return _result(0, {}, "network", "Couldn't reach the accounts server")
	var response: Array = await http.request_completed
	http.queue_free()
	var outcome: int = response[0]
	var status: int = response[1]
	var raw: PackedByteArray = response[3]
	if outcome != HTTPRequest.RESULT_SUCCESS:
		return _result(0, {}, "network", "Couldn't reach the accounts server")
	var data := {}
	if raw.size() > 0:
		var parsed: Variant = JSON.parse_string(raw.get_string_from_utf8())
		if parsed is Dictionary:
			data = parsed
	if status == 401 and has_session():
		clear_session()
	if status >= 200 and status < 300:
		return _result(status, data, "", "")
	var message := str(data.get("message", "Request failed (%d)" % status))
	return _result(status, data, str(data.get("error", "http_%d" % status)), message)


func sign_up(email: String, password: String, display_name: String) -> Dictionary:
	var body := {"email": email, "password": password, "display_name": display_name}
	return await call_api(HTTPClient.METHOD_POST, "/auth/signup", body)


func log_in(email: String, password: String) -> Dictionary:
	var body := {"email": email, "password": password}
	return _keep_session(await call_api(HTTPClient.METHOD_POST, "/auth/login", body))


func verify_email(token: String) -> Dictionary:
	var body := {"token": token}
	return _keep_session(await call_api(HTTPClient.METHOD_POST, "/auth/verify-email", body))


func request_reset(email: String) -> Dictionary:
	var body := {"email": email}
	return await call_api(HTTPClient.METHOD_POST, "/auth/password-reset/request", body)


func confirm_reset(token: String, password: String) -> Dictionary:
	var body := {"token": token, "password": password}
	var path := "/auth/password-reset/confirm"
	return _keep_session(await call_api(HTTPClient.METHOD_POST, path, body))


func log_out() -> void:
	if has_session():
		await call_api(HTTPClient.METHOD_POST, "/auth/logout")
	clear_session()


func me() -> Dictionary:
	return await call_api(HTTPClient.METHOD_GET, "/me")


func set_display_name(display_name: String) -> Dictionary:
	var body := {"display_name": display_name}
	return await call_api(HTTPClient.METHOD_PUT, "/me/display-name", body)


func join_ticket() -> Dictionary:
	return await call_api(HTTPClient.METHOD_POST, "/join-ticket")


## Starts Discord sign-in (or linking, with `link`). Keeps a random verifier locally
## and sends only its hash, so only this browser can redeem the code Discord leads to.
## On success, data["url"] is the Discord page to open.
func start_discord(link: bool) -> Dictionary:
	var verifier := Marshalls.raw_to_base64(Crypto.new().generate_random_bytes(32))
	save_value(VERIFIER_KEY, verifier)
	var body := {"code_challenge": verifier.sha256_text(), "link": link}
	return await call_api(HTTPClient.METHOD_POST, "/auth/discord/start", body)


func finish_discord(code: String) -> Dictionary:
	var verifier := load_value(VERIFIER_KEY)
	save_value(VERIFIER_KEY, "")
	var body := {"code": code, "code_verifier": verifier}
	return _keep_session(await call_api(HTTPClient.METHOD_POST, "/auth/discord/exchange", body))


func _keep_session(result: Dictionary) -> Dictionary:
	if result["ok"]:
		set_session(str((result["data"] as Dictionary).get("token", "")))
	return result


func _result(status: int, data: Dictionary, error: String, message: String) -> Dictionary:
	return {
		"ok": error.is_empty(), "status": status, "data": data, "error": error, "message": message
	}


## Small persistent key/value store: localStorage on the web, a ConfigFile natively.
static func load_value(key: String) -> String:
	if OS.has_feature("web"):
		var value: Variant = JavaScriptBridge.eval(
			"window.localStorage.getItem(%s) || ''" % JSON.stringify(key)
		)
		return str(value) if value != null else ""
	var config := ConfigFile.new()
	if config.load(NATIVE_STORE) != OK:
		return ""
	return str(config.get_value("account", key, ""))


static func save_value(key: String, value: String) -> void:
	if OS.has_feature("web"):
		if value.is_empty():
			JavaScriptBridge.eval("window.localStorage.removeItem(%s)" % JSON.stringify(key))
		else:
			JavaScriptBridge.eval(
				"window.localStorage.setItem(%s, %s)" % [JSON.stringify(key), JSON.stringify(value)]
			)
		return
	var config := ConfigFile.new()
	config.load(NATIVE_STORE)
	config.set_value("account", key, value)
	config.save(NATIVE_STORE)
