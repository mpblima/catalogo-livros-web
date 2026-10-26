package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRetailerCoverRequiresExactISBNAndStaysOnRetailer(t *testing.T) {
	page := `<meta property="og:image" content="https://cdn.example.com/capa.jpg"><span>ISBN: 9780134685991</span>`
	if productCover(page, "https://loja.example.com/produto/livro", "9780134685991") == "" {
		t.Fatal("exact edition rejected")
	}
	if productCover(page, "https://loja.example.com/produto/livro", "9780134685992") != "" {
		t.Fatal("wrong edition accepted")
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/busca":
			w.Write([]byte(`<a href="http://127.0.0.1:1/produto/no">bad</a><a href="/produto/exato">ok</a>`))
		case "/produto/exato":
			w.Write([]byte(page))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()
	l := lookupClient{client: server.Client(), open: server.URL}
	if got := l.retailerCover(context.Background(), "9780134685991", "Livro"); got != "https://cdn.example.com/capa.jpg" {
		t.Fatal(got)
	}
}
