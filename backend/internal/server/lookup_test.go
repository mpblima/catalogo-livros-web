package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestISBNNormalization(t *testing.T) {
	for raw, want := range map[string]string{"0-13-468599-7": "9780134685991", "9780134685991": "9780134685991", "978013468599112345": "9780134685991", "097522980X": "9780975229804"} {
		got, err := isbnCode(raw)
		if err != nil || got != want {
			t.Errorf("%s: got %s %v", raw, got, err)
		}
	}
	for _, raw := range []string{"9780134685992", "123", "1234567890123", "XXXXXXXXXX"} {
		if _, err := isbnCode(raw); err == nil {
			t.Errorf("accepted invalid ISBN %s", raw)
		}
	}
}
func TestLookupFallbackOnlyAcceptsExactISBN(t *testing.T) {
	hits := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/books":
			w.WriteHeader(429)
			fmt.Fprint(w, `{}`)
		case "/volumes":
			hits++
			fmt.Fprint(w, `{"items":[{"volumeInfo":{"title":"Wrong edition","industryIdentifiers":[{"identifier":"9780132350884"}]}},{"volumeInfo":{"title":"Effective Java","authors":["Joshua Bloch"],"industryIdentifiers":[{"identifier":"0134685997"}],"pageCount":416,"description":"<p>Review &amp; save</p>","imageLinks":{"thumbnail":"http://books.google.com/cover.jpg"}}}]}`)
		}
	}))
	defer ts.Close()
	l := lookupClient{client: ts.Client(), open: ts.URL, google: ts.URL}
	b, err := l.find(context.Background(), "9780134685991")
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "Effective Java" || b.Metadata["author"] != "Joshua Bloch" || b.Metadata["pages"] != "416" || b.Summary != "Review & save" || b.CoverURL != "https://books.google.com/cover.jpg" || hits != 1 {
		t.Fatalf("bad draft: %#v", b)
	}
}
func TestLookupEnrichesOpenLibrary(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/books":
			fmt.Fprint(w, `{"ISBN:9780134685991":{"title":"Effective Java","key":"/books/OL1M","authors":[{"name":"Joshua Bloch"}],"cover":{"large":"https://covers.openlibrary.org/test.jpg"}}}`)
		case "/books/OL1M.json":
			fmt.Fprint(w, `{"edition_name":"3rd","languages":[{"key":"/languages/eng"}],"works":[{"key":"/works/OL2W"}]}`)
		case "/works/OL2W.json":
			fmt.Fprint(w, `{"description":{"value":"A reference"}}`)
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	defer ts.Close()
	b, err := (lookupClient{client: ts.Client(), open: ts.URL, google: ts.URL}).find(context.Background(), "0134685997")
	if err != nil {
		t.Fatal(err)
	}
	if b.Metadata["edition"] != "3rd" || b.Metadata["language"] != "eng" || b.Summary != "A reference" || b.Metadata["isbn"] != "9780134685991" {
		t.Fatalf("bad enrichment %#v", b)
	}
}
func TestLookupNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{}`) }))
	defer ts.Close()
	_, err := (lookupClient{client: ts.Client(), open: ts.URL, google: ts.URL}).find(context.Background(), "9780134685991")
	if e, ok := err.(apiError); !ok || e.Code != 404 {
		t.Fatalf("want not found, got %v", err)
	}
}

// The live smoke is opt-in: public providers must not make CI nondeterministic.
func TestLiveISBN(t *testing.T) {
	if os.Getenv("LIVE_ISBN_SMOKE") != "1" {
		t.Skip("opt-in public provider smoke")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 28*time.Second)
	defer cancel()
	b, err := (lookupClient{client: &http.Client{Timeout: 8 * time.Second}, open: "https://openlibrary.org", google: "https://www.googleapis.com/books/v1", links: "https://books.google.com/books"}).find(ctx, "9780134685991")
	if err != nil {
		t.Fatal(err)
	}
	if b.Title == "" || b.Metadata["isbn"] != "9780134685991" {
		t.Fatalf("unexpected draft %#v", b)
	}
	t.Logf("title=%s author=%s pages=%s cover=%t source=%s", b.Title, b.Metadata["author"], b.Metadata["pages"], b.CoverURL != "", b.Metadata["source"])
}

type lookupRoundTrip func(*http.Request) (*http.Response, error)

func (f lookupRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestGooglePageFallback(t *testing.T) {
	client := &http.Client{Transport: lookupRoundTrip(func(r *http.Request) (*http.Response, error) {
		body := `{}`
		if r.URL.Query().Get("callback") == "bookInfo" {
			body = `bookInfo({"ISBN:9780134685991":{"bib_key":"ISBN:9780134685991","info_url":"https://books.google.com/books?id=exact","thumbnail_url":"https://books.google.com/cover.jpg"}});`
		}
		if r.URL.Query().Get("id") == "exact" {
			body = `<html><meta content="An exact &amp; useful edition" name="description"><table><td class="metadata_label">Title</td><td class="metadata_value"><span>Effective Java</span></td><td class="metadata_label">Author</td><td class="metadata_value">Joshua Bloch</td><td class="metadata_label">Edition</td><td class="metadata_value">3</td><td class="metadata_label">Length</td><td class="metadata_value">416 pages</td></table></html>`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})}
	b, err := (lookupClient{client: client, open: "https://openlibrary.org", google: "https://www.googleapis.com/books/v1", links: "https://books.google.com/books"}).find(context.Background(), "9780134685991")
	if err != nil {
		t.Fatal(err)
	}
	if b.Title != "Effective Java" || b.Metadata["edition"] != "3" || b.Metadata["pages"] != "416" || b.Summary != "An exact & useful edition" {
		t.Fatalf("bad page fallback: %#v", b)
	}
}
