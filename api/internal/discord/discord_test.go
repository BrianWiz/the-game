package discord

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestExchange(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/oauth2/token", func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		r.ParseForm()
		if user != "cid" || pass != "secret" || r.Form.Get("code") != "the-code" ||
			r.Form.Get("redirect_uri") != "https://x/cb" {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		w.Write([]byte(`{"access_token":"at","token_type":"Bearer"}`))
	})
	mux.HandleFunc("GET /api/users/@me", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer at" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Write([]byte(`{"id":"123","username":"bob"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	o := &OAuth{ClientID: "cid", ClientSecret: "secret", RedirectURI: "https://x/cb", BaseURL: srv.URL}
	u, err := o.Exchange(context.Background(), "the-code")
	if err != nil {
		t.Fatal(err)
	}
	if u != (User{ID: "123", Username: "bob"}) {
		t.Fatalf("user = %+v", u)
	}
	if _, err := o.Exchange(context.Background(), "wrong"); err == nil {
		t.Fatal("bad code accepted")
	}
}

func TestAuthURL(t *testing.T) {
	o := &OAuth{ClientID: "cid", RedirectURI: "https://x/cb"}
	u, err := url.Parse(o.AuthURL("st"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Host != "discord.com" || q.Get("state") != "st" || q.Get("scope") != "identify" ||
		q.Get("redirect_uri") != "https://x/cb" || q.Get("client_id") != "cid" {
		t.Fatalf("auth url = %s", u)
	}
}
