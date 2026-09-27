package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tfpp/the-game/api/internal/auth"
	"github.com/tfpp/the-game/api/internal/discord"
	"github.com/tfpp/the-game/api/internal/mail"
	"github.com/tfpp/the-game/api/internal/store"
	"github.com/tfpp/the-game/api/internal/ticket"
)

var testKey = []byte("test-ticket-key-0123456789abcdef")

type fakeMailer struct {
	mu   sync.Mutex
	sent []mail.Message
}

func (f *fakeMailer) Send(_ context.Context, m mail.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, m)
	return nil
}

func (f *fakeMailer) take() []mail.Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.sent
	f.sent = nil
	return out
}

type fakeDiscord struct {
	users map[string]discord.User // code -> user
}

func (f *fakeDiscord) AuthURL(state string) string {
	return "https://discord.test/authorize?state=" + url.QueryEscape(state)
}

func (f *fakeDiscord) Exchange(_ context.Context, code string) (discord.User, error) {
	u, ok := f.users[code]
	if !ok {
		return discord.User{}, errors.New("bad code")
	}
	return u, nil
}

type harness struct {
	t    *testing.T
	srv  *Server
	h    http.Handler
	mail *fakeMailer
	disc *fakeDiscord
	now  time.Time
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	h := &harness{t: t, mail: &fakeMailer{}, disc: &fakeDiscord{users: map[string]discord.User{}}}
	h.now = time.Unix(1_800_000_000, 0)
	h.srv = New(Config{
		PublicURL:      "https://game.test",
		ClientURL:      "https://client.test/the-game/",
		AllowedOrigins: []string{"https://client.test"},
		TicketKey:      testKey,
		Logger:         slog.New(slog.NewTextHandler(io.Discard, nil)),
		Now:            func() time.Time { return h.now },
	}, st, h.mail, h.disc)
	h.h = h.srv.Handler()
	return h
}

type response struct {
	code   int
	header http.Header
	body   map[string]any
}

func (h *harness) do(method, path, token string, body any) response {
	h.t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, r)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, req)
	out := response{code: rec.Code, header: rec.Header(), body: map[string]any{}}
	json.Unmarshal(rec.Body.Bytes(), &out.body)
	return out
}

func (r response) str(key string) string { s, _ := r.body[key].(string); return s }

func (r response) account() map[string]any { a, _ := r.body["account"].(map[string]any); return a }

func (h *harness) expect(r response, code int, errCode string) {
	h.t.Helper()
	if r.code != code || (errCode != "" && r.str("error") != errCode) {
		h.t.Fatalf("got %d %v, want %d %q", r.code, r.body, code, errCode)
	}
}

var fragment = regexp.MustCompile(`#(\w+)=([A-Za-z0-9_\-%]+)`)

// linkToken extracts the token from the single email that was sent.
func (h *harness) linkToken(kind string) string {
	h.t.Helper()
	h.srv.WaitMail()
	msgs := h.mail.take()
	if len(msgs) != 1 {
		h.t.Fatalf("sent %d emails, want 1", len(msgs))
	}
	m := fragment.FindStringSubmatch(msgs[0].Text)
	if m == nil || m[1] != kind {
		h.t.Fatalf("no #%s= link in %q", kind, msgs[0].Text)
	}
	return m[2]
}

// signUp creates a verified email account and returns its session token.
func (h *harness) signUp(email, password, name string) string {
	h.t.Helper()
	h.expect(h.do("POST", "/api/auth/signup", "", map[string]string{
		"email": email, "password": password, "display_name": name,
	}), http.StatusAccepted, "")
	r := h.do("POST", "/api/auth/verify-email", "", map[string]string{"token": h.linkToken("verify")})
	h.expect(r, http.StatusOK, "")
	return r.str("token")
}

func TestSignupVerifyLoginFlow(t *testing.T) {
	h := newHarness(t)
	token := h.signUp("Alice@Example.com ", "correct horse", "Alice")

	me := h.do("GET", "/api/me", token, nil)
	h.expect(me, http.StatusOK, "")
	if me.str("email") != "alice@example.com" || me.str("display_name") != "Alice" {
		t.Fatalf("me = %v", me.body)
	}

	h.expect(h.do("POST", "/api/auth/login", "", map[string]string{
		"email": "alice@example.com", "password": "wrong password",
	}), http.StatusUnauthorized, "invalid_credentials")
	login := h.do("POST", "/api/auth/login", "", map[string]string{
		"email": "ALICE@example.com", "password": "correct horse",
	})
	h.expect(login, http.StatusOK, "")

	h.expect(h.do("POST", "/api/auth/logout", login.str("token"), nil), http.StatusNoContent, "")
	h.expect(h.do("GET", "/api/me", login.str("token"), nil), http.StatusUnauthorized, "unauthorized")
	h.expect(h.do("GET", "/api/me", token, nil), http.StatusOK, "") // other session unaffected
}

func TestLoginUnknownEmailLooksLikeWrongPassword(t *testing.T) {
	h := newHarness(t)
	h.expect(h.do("POST", "/api/auth/login", "", map[string]string{
		"email": "nobody@example.com", "password": "whatever1",
	}), http.StatusUnauthorized, "invalid_credentials")
}

func TestVerifyTokenIsSingleUseAndExpires(t *testing.T) {
	h := newHarness(t)
	body := map[string]string{"email": "a@example.com", "password": "password1", "display_name": "Aaa"}
	h.expect(h.do("POST", "/api/auth/signup", "", body), http.StatusAccepted, "")
	tok := h.linkToken("verify")
	h.now = h.now.Add(signupTTL + time.Second)
	h.expect(h.do("POST", "/api/auth/verify-email", "", map[string]string{"token": tok}), http.StatusBadRequest, "invalid_token")

	h.expect(h.do("POST", "/api/auth/signup", "", body), http.StatusAccepted, "")
	tok = h.linkToken("verify")
	h.expect(h.do("POST", "/api/auth/verify-email", "", map[string]string{"token": tok}), http.StatusOK, "")
	h.expect(h.do("POST", "/api/auth/verify-email", "", map[string]string{"token": tok}), http.StatusBadRequest, "invalid_token")
}

func TestSignupDoesNotRevealExistingEmail(t *testing.T) {
	h := newHarness(t)
	h.signUp("a@example.com", "password1", "First")
	r := h.do("POST", "/api/auth/signup", "", map[string]string{
		"email": "a@example.com", "password": "attacker-pw", "display_name": "Second",
	})
	h.expect(r, http.StatusAccepted, "")
	h.srv.WaitMail()
	msgs := h.mail.take()
	if len(msgs) != 1 || strings.Contains(msgs[0].Text, "#verify=") {
		t.Fatalf("existing address should get a notice, not a verify link: %v", msgs)
	}
	// The original password still works.
	h.expect(h.do("POST", "/api/auth/login", "", map[string]string{
		"email": "a@example.com", "password": "password1",
	}), http.StatusOK, "")
}

// Signing up with someone else's address must not let the attacker set their password:
// each verification link carries the password from its own sign-up.
func TestPendingSignupCannotPresetPassword(t *testing.T) {
	h := newHarness(t)
	h.expect(h.do("POST", "/api/auth/signup", "", map[string]string{
		"email": "v@example.com", "password": "attacker-pw", "display_name": "Evil",
	}), http.StatusAccepted, "")
	attackerLink := h.linkToken("verify")
	_ = attackerLink // delivered to the victim's inbox; the victim ignores it
	h.expect(h.do("POST", "/api/auth/signup", "", map[string]string{
		"email": "v@example.com", "password": "victim-pw1", "display_name": "Victim",
	}), http.StatusAccepted, "")
	h.expect(h.do("POST", "/api/auth/verify-email", "", map[string]string{"token": h.linkToken("verify")}), http.StatusOK, "")
	h.expect(h.do("POST", "/api/auth/login", "", map[string]string{
		"email": "v@example.com", "password": "attacker-pw",
	}), http.StatusUnauthorized, "invalid_credentials")
}

func TestSignupValidation(t *testing.T) {
	h := newHarness(t)
	h.signUp("a@example.com", "password1", "Taken")
	cases := []struct {
		email, password, name, code string
		status                      int
	}{
		{"not-an-email", "password1", "Name1", "invalid_email", 400},
		{"b@example.com", "short", "Name1", "invalid_password", 400},
		{"b@example.com", "password1", "no spaces", "invalid_name", 400},
		{"b@example.com", "password1", "ab", "invalid_name", 400},
		{"b@example.com", "password1", "Admin", "invalid_name", 400},
		{"b@example.com", "password1", "tAKEN", "name_taken", 409},
	}
	for _, c := range cases {
		h.expect(h.do("POST", "/api/auth/signup", "", map[string]string{
			"email": c.email, "password": c.password, "display_name": c.name,
		}), c.status, c.code)
	}
}

func TestDisplayNamesAreCaseInsensitivelyUnique(t *testing.T) {
	h := newHarness(t)
	a := h.signUp("a@example.com", "password1", "Alpha")
	b := h.signUp("b@example.com", "password1", "Bravo")
	h.expect(h.do("PUT", "/api/me/display-name", b, map[string]string{"display_name": "ALPHA"}), http.StatusConflict, "name_taken")
	r := h.do("PUT", "/api/me/display-name", a, map[string]string{"display_name": "alpha_2"})
	h.expect(r, http.StatusOK, "")
	if r.str("display_name") != "alpha_2" {
		t.Fatalf("rename = %v", r.body)
	}
	h.expect(h.do("PUT", "/api/me/display-name", b, map[string]string{"display_name": "Alpha"}), http.StatusOK, "")
}

// If a name was claimed between sign-up and verification, the account is still created
// and the player picks another name.
func TestVerifyWithNameTakenMeanwhile(t *testing.T) {
	h := newHarness(t)
	h.expect(h.do("POST", "/api/auth/signup", "", map[string]string{
		"email": "a@example.com", "password": "password1", "display_name": "Racer",
	}), http.StatusAccepted, "")
	tok := h.linkToken("verify")
	h.signUp("b@example.com", "password1", "racer")
	r := h.do("POST", "/api/auth/verify-email", "", map[string]string{"token": tok})
	h.expect(r, http.StatusOK, "")
	if r.account()["display_name"] != "" {
		t.Fatalf("account = %v", r.account())
	}
	h.expect(h.do("POST", "/api/join-ticket", r.str("token"), nil), http.StatusConflict, "display_name_required")
}

func TestPasswordReset(t *testing.T) {
	h := newHarness(t)
	old := h.signUp("a@example.com", "password1", "Alice")

	h.expect(h.do("POST", "/api/auth/password-reset/request", "", map[string]string{"email": "nobody@example.com"}), http.StatusAccepted, "")
	h.srv.WaitMail()
	if n := len(h.mail.take()); n != 0 {
		t.Fatalf("mailed an unknown address (%d)", n)
	}

	h.expect(h.do("POST", "/api/auth/password-reset/request", "", map[string]string{"email": "a@example.com"}), http.StatusAccepted, "")
	tok := h.linkToken("reset")
	h.expect(h.do("POST", "/api/auth/password-reset/confirm", "", map[string]string{"token": tok, "password": "short"}), http.StatusBadRequest, "invalid_password")
	r := h.do("POST", "/api/auth/password-reset/confirm", "", map[string]string{"token": tok, "password": "new-password"})
	h.expect(r, http.StatusOK, "")
	h.expect(h.do("GET", "/api/me", old, nil), http.StatusUnauthorized, "unauthorized")
	h.expect(h.do("GET", "/api/me", r.str("token"), nil), http.StatusOK, "")
	h.expect(h.do("POST", "/api/auth/password-reset/confirm", "", map[string]string{"token": tok, "password": "again-password"}), http.StatusBadRequest, "invalid_token")
	h.expect(h.do("POST", "/api/auth/login", "", map[string]string{"email": "a@example.com", "password": "new-password"}), http.StatusOK, "")
}

// discordLogin runs start -> callback -> exchange and returns the exchange response.
func (h *harness) discordLogin(code, sessionToken string, link bool) response {
	h.t.Helper()
	h.now = h.now.Add(time.Minute) // refill the per-IP limiter; each call makes several requests
	verifier := auth.NewToken()
	start := h.do("POST", "/api/auth/discord/start", sessionToken, map[string]any{
		"code_challenge": auth.CodeChallenge(verifier), "link": link,
	})
	h.expect(start, http.StatusOK, "")
	u, _ := url.Parse(start.str("url"))
	state := u.Query().Get("state")

	cb := h.do("GET", "/api/auth/discord/callback?code="+code+"&state="+url.QueryEscape(state), "", nil)
	if cb.code != http.StatusFound {
		h.t.Fatalf("callback status %d", cb.code)
	}
	loc := cb.header.Get("Location")
	m := fragment.FindStringSubmatch(loc)
	if m == nil || m[1] != "discord_code" {
		return response{code: cb.code, body: map[string]any{"redirect": loc}}
	}
	loginCode, _ := url.QueryUnescape(m[2])
	// A different browser (wrong verifier) can't redeem the code...
	h.expect(h.do("POST", "/api/auth/discord/exchange", "", map[string]string{
		"code": loginCode, "code_verifier": "someone-else",
	}), http.StatusBadRequest, "invalid_code")
	// ...and the failed attempt burned it, so run the flow once more for real.
	start = h.do("POST", "/api/auth/discord/start", sessionToken, map[string]any{
		"code_challenge": auth.CodeChallenge(verifier), "link": link,
	})
	u, _ = url.Parse(start.str("url"))
	cb = h.do("GET", "/api/auth/discord/callback?code="+code+"&state="+url.QueryEscape(u.Query().Get("state")), "", nil)
	m = fragment.FindStringSubmatch(cb.header.Get("Location"))
	loginCode, _ = url.QueryUnescape(m[2])
	return h.do("POST", "/api/auth/discord/exchange", "", map[string]string{
		"code": loginCode, "code_verifier": verifier,
	})
}

func TestDiscordSignupPicksNameThenGetsTicket(t *testing.T) {
	h := newHarness(t)
	h.disc.users["c1"] = discord.User{ID: "111", Username: "discord_bob"}
	r := h.discordLogin("c1", "", false)
	h.expect(r, http.StatusOK, "")
	token := r.str("token")
	acct := r.account()
	if acct["display_name"] != "" || acct["discord_linked"] != true {
		t.Fatalf("account = %v", acct)
	}
	h.expect(h.do("POST", "/api/join-ticket", token, nil), http.StatusConflict, "display_name_required")
	h.expect(h.do("PUT", "/api/me/display-name", token, map[string]string{"display_name": "Bob"}), http.StatusOK, "")

	tr := h.do("POST", "/api/join-ticket", token, nil)
	h.expect(tr, http.StatusOK, "")
	claims, err := ticket.Verify(testKey, tr.str("ticket"), h.now)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Name != "Bob" || claims.AccountID != int64(acct["id"].(float64)) ||
		claims.Expires != h.now.Add(ticketTTL).Unix() || claims.Nonce == "" {
		t.Fatalf("claims = %+v", claims)
	}

	// Signing in again with the same Discord identity reaches the same account.
	again := h.discordLogin("c1", "", false)
	h.expect(again, http.StatusOK, "")
	if again.account()["id"] != acct["id"] || again.account()["display_name"] != "Bob" {
		t.Fatalf("second login = %v", again.account())
	}
}

func TestDiscordLinkToEmailAccount(t *testing.T) {
	h := newHarness(t)
	h.disc.users["c1"] = discord.User{ID: "111", Username: "bob"}
	h.disc.users["c2"] = discord.User{ID: "222", Username: "carol"}
	email := h.signUp("a@example.com", "password1", "Alice")
	me := h.do("GET", "/api/me", email, nil)

	linked := h.discordLogin("c1", email, true)
	h.expect(linked, http.StatusOK, "")
	if linked.account()["id"] != me.body["id"] || linked.account()["discord_linked"] != true {
		t.Fatalf("link = %v", linked.account())
	}
	// Discord sign-in now reaches the email account.
	h.expect(h.discordLogin("c1", "", false), http.StatusOK, "")
	if got := h.discordLogin("c1", "", false).account()["email"]; got != "a@example.com" {
		t.Fatalf("discord login reached %v", got)
	}

	// An account keeps its first Discord identity.
	r := h.discordLogin("c2", email, true)
	if !strings.Contains(r.str("redirect"), "auth_error=discord_already_linked") {
		t.Fatalf("relink = %v", r.body)
	}
	// A Discord identity already used by another account can't be linked.
	bob := h.signUp("b@example.com", "password1", "Bobby")
	r = h.discordLogin("c1", bob, true)
	if !strings.Contains(r.str("redirect"), "auth_error=discord_in_use") {
		t.Fatalf("steal link = %v", r.body)
	}
}

func TestDiscordLinkRequiresSession(t *testing.T) {
	h := newHarness(t)
	h.expect(h.do("POST", "/api/auth/discord/start", "", map[string]any{
		"code_challenge": auth.CodeChallenge("v"), "link": true,
	}), http.StatusUnauthorized, "unauthorized")
}

func TestDiscordCallbackRejectsUnknownState(t *testing.T) {
	h := newHarness(t)
	r := h.do("GET", "/api/auth/discord/callback?code=c1&state=forged", "", nil)
	if r.code != http.StatusFound || !strings.HasSuffix(r.header.Get("Location"), "#auth_error=discord_expired") {
		t.Fatalf("callback = %d %s", r.code, r.header.Get("Location"))
	}
}

func TestDisabledProviders(t *testing.T) {
	h := newHarness(t)
	h.srv.mailer = nil
	h.srv.discord = nil
	h.expect(h.do("POST", "/api/auth/signup", "", map[string]string{}), http.StatusServiceUnavailable, "email_disabled")
	h.expect(h.do("POST", "/api/auth/discord/start", "", map[string]string{"code_challenge": auth.CodeChallenge("v")}),
		http.StatusServiceUnavailable, "discord_disabled")
	health := h.do("GET", "/api/health", "", nil)
	h.expect(health, http.StatusOK, "")
	if health.body["email"] != false || health.body["discord"] != false {
		t.Fatalf("health = %v", health.body)
	}
}

func TestLoginRateLimitPerEmail(t *testing.T) {
	h := newHarness(t)
	body := map[string]string{"email": "a@example.com", "password": "password1"}
	for range 10 {
		h.expect(h.do("POST", "/api/auth/login", "", body), http.StatusUnauthorized, "")
	}
	r := h.do("POST", "/api/auth/login", "", body)
	h.expect(r, http.StatusTooManyRequests, "rate_limited")
	if r.header.Get("Retry-After") == "" {
		t.Fatal("missing Retry-After")
	}
	h.now = h.now.Add(time.Minute)
	h.expect(h.do("POST", "/api/auth/login", "", body), http.StatusUnauthorized, "")
}

func TestCORS(t *testing.T) {
	h := newHarness(t)
	pre := httptest.NewRequest("OPTIONS", "/api/join-ticket", nil)
	pre.Header.Set("Origin", "https://client.test")
	pre.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()
	h.h.ServeHTTP(rec, pre)
	if rec.Code != http.StatusNoContent || rec.Header().Get("Access-Control-Allow-Origin") != "https://client.test" ||
		!strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), "Authorization") {
		t.Fatalf("preflight = %d %v", rec.Code, rec.Header())
	}
	if rec.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatal("credentials must not be allowed")
	}

	evil := httptest.NewRequest("GET", "/api/health", nil)
	evil.Header.Set("Origin", "https://evil.test")
	rec = httptest.NewRecorder()
	h.h.ServeHTTP(rec, evil)
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("foreign origin allowed")
	}
}

func TestUnknownRouteIs404JSON(t *testing.T) {
	h := newHarness(t)
	h.expect(h.do("GET", "/api/nope", "", nil), http.StatusNotFound, "not_found")
}
