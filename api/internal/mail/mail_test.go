package mail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResendSend(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer re_test" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"id":"abc"}`))
	}))
	defer srv.Close()

	r := &Resend{APIKey: "re_test", From: "Game <noreply@example.com>", Endpoint: srv.URL}
	if err := r.Send(context.Background(), Message{To: "a@example.com", Subject: "Hi", Text: "Body"}); err != nil {
		t.Fatal(err)
	}
	if got["from"] != "Game <noreply@example.com>" || got["subject"] != "Hi" || got["text"] != "Body" {
		t.Fatalf("payload = %v", got)
	}
	if to, _ := got["to"].([]any); len(to) != 1 || to[0] != "a@example.com" {
		t.Fatalf("to = %v", got["to"])
	}
}

func TestResendError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"domain not verified"}`, http.StatusForbidden)
	}))
	defer srv.Close()
	r := &Resend{APIKey: "k", From: "f@example.com", Endpoint: srv.URL}
	if err := r.Send(context.Background(), Message{To: "a@example.com"}); err == nil {
		t.Fatal("expected error")
	}
}
