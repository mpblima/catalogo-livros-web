package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestResendAPI(t *testing.T) {
	count := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if r.Header.Get("Authorization") != "Bearer secret-test" {
			t.Error("missing provider authentication")
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body["subject"] != "Convite" {
			t.Error("wrong subject")
		}
		if body["from"] != "Biblioteca <convites@example.com>" {
			t.Errorf("wrong sender: %v", body["from"])
		}
		to, ok := body["to"].([]any)
		if !ok || len(to) != 1 || to[0] != "guest@example.com" {
			t.Errorf("wrong recipient: %v", body["to"])
		}
		if count == 1 {
			w.WriteHeader(429)
		} else {
			w.WriteHeader(202)
		}
	}))
	defer srv.Close()
	m := ResendMailer{APIKey: "secret-test", FromEmail: "convites@example.com", FromName: "Biblioteca", Endpoint: srv.URL}
	message := Message{To: "guest@example.com", Subject: "Convite", HTML: "<p>Convite</p>"}
	if err := m.Send(context.Background(), message); err == nil {
		t.Fatal("provider failure ignored")
	}
	if err := m.Send(context.Background(), message); err != nil {
		t.Fatal(err)
	}
}
