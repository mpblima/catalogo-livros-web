package server

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestVerifiedIdentity(t *testing.T) {
	id := "mail1"
	u := &clerk.User{ID: "u1", PrimaryEmailAddressID: &id, EmailAddresses: []*clerk.EmailAddress{{ID: id, EmailAddress: "Owner@Example.com", Verification: &clerk.Verification{Status: "unverified"}}}}
	if _, err := verifiedIdentity(u); err == nil {
		t.Fatal("unverified email accepted")
	}
	u.EmailAddresses[0].Verification.Status = "verified"
	a, err := verifiedIdentity(u)
	if err != nil || a.Email != "owner@example.com" {
		t.Fatalf("bad identity: %#v %v", a, err)
	}
	u.Banned = true
	if _, err := verifiedIdentity(u); err == nil {
		t.Fatal("banned user accepted")
	}
}
func TestIntegration(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL must point at a disposable migrated database")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	_, err = pool.Exec(context.Background(), `truncate catalog.erased_accounts,catalog.email_outbox,catalog.audit_events,catalog.privacy_requests,catalog.comments,catalog.sync_links,catalog.books,catalog.members,catalog.catalogs,catalog.access_grants,catalog.users cascade;update catalog.service_settings set initialized=false`)
	if err != nil {
		t.Fatal(err)
	}
	deleteCalls := 0
	s := Server{DeleteIdentity: func(ctx context.Context, id string) error {
		deleteCalls++
		if deleteCalls == 1 {
			return fmt.Errorf("provider unavailable")
		}
		return nil
	}, DB: pool, Mailer: testMailer{}, Config: Config{BootstrapAdminEmail: "owner@example.com", AutoActivate: true, Origins: []string{"http://localhost:5173"}, FrontendURL: "http://localhost:5173"}, Auth: func(ctx context.Context, token string) (Identity, error) {
		if token == "bad" {
			return Identity{}, fmt.Errorf("invalid")
		}
		return Identity{token, token + "@example.com", token}, nil
	}}
	handler := s.Handler()
	req := func(token, method, path, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	expect := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("want %d got %d: %s", status, w.Code, w.Body.String())
		}
	}
	for _, endpoint := range []struct{ Method, Path string }{{"GET", "/api/catalogs"}, {"POST", "/api/catalogs/no/books"}, {"PUT", "/api/catalogs/no/books/no"}, {"DELETE", "/api/catalogs/no/books/no"}, {"POST", "/api/catalogs/no/books/no/comments"}, {"POST", "/api/catalogs/no/members"}, {"PUT", "/api/admin/access"}, {"GET", "/api/privacy/export"}, {"POST", "/api/privacy/erasure"}} {
		expect(req("", endpoint.Method, endpoint.Path, "{}"), 401)
	}
	corsReq := httptest.NewRequest("GET", "/api/me", nil)
	corsReq.Header.Set("Origin", "https://attacker.example")
	corsResp := httptest.NewRecorder()
	handler.ServeHTTP(corsResp, corsReq)
	expect(corsResp, 403)
	expect(req("", "GET", "/api/me", ""), 401)
	expect(req("bad", "GET", "/api/me", ""), 401)
	owner := req("owner", "GET", "/api/me", "")
	expect(owner, 200)
	if !strings.Contains(owner.Body.String(), `"admin":true`) {
		t.Fatal(owner.Body.String())
	}
	catalogFor := func(user string) string {
		t.Helper()
		w := req(user, "GET", "/api/catalogs", "")
		expect(w, 200)
		var list []struct {
			ID string `json:"id"`
		}
		if err = json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list) == 0 {
			t.Fatalf("no catalog %s", w.Body.String())
		}
		return list[0].ID
	}
	id := catalogFor("owner")
	other := catalogFor("other")
	physical := req("owner", "POST", "/api/catalogs/"+id+"/books", `{"title":"Livro físico","kind":"physical","metadata":{"isbn":"0134685997","subtitle":"Subtítulo","pages":"416","page":"12","notes":"Minha nota","vendor":{"code":"42"}}}`)
	expect(physical, 201)
	if !strings.Contains(physical.Body.String(), `"isbn":"9780134685991"`) || !strings.Contains(physical.Body.String(), `"vendor":{"code":"42"}`) {
		t.Fatal(physical.Body.String())
	}
	filtered := req("owner", "GET", "/api/catalogs/"+id+"/books?kind=digital", "")
	expect(filtered, 200)
	if strings.TrimSpace(filtered.Body.String()) != "[]" {
		t.Fatal("physical books leaked into digital filter: " + filtered.Body.String())
	}
	expect(req("owner", "GET", "/api/catalogs/"+id+"/books?kind=other", ""), 400)

	base := "/api/catalogs/" + id
	expect(req("other", "GET", base+"/books", ""), 404)
	expect(req("other", "PUT", "/api/admin/access", `{"email":"other@example.com","enabled":true,"admin":true}`), 403)
	b := req("owner", "POST", base+"/books", `{"title":"Livro Java","kind":"digital","path":"/home/owner/Livros/java.pdf","metadata":{"author":"Autora"},"favorite":true}`)
	expect(b, 201)
	var book Book
	_ = json.Unmarshal(b.Body.Bytes(), &book)
	expect(req("owner", "POST", base+"/categories", `{"path":"livros_fisicos/estudos/antigo"}`), 200)
	expect(req("other", "POST", base+"/categories", `{"path":"livros_fisicos/invasao"}`), 404)
	expect(req("owner", "POST", base+"/categories", `{"path":"livros_fisicos/ .. /invalido"}`), 400)
	expect(req("owner", "PUT", base+"/categories", `{"path":"livros_fisicos/estudos","new_path":"livros_fisicos/referencia"}`), 200)
	categories := req("owner", "GET", base+"/categories", "")
	expect(categories, 200)
	if !strings.Contains(categories.Body.String(), "livros_fisicos/referencia/antigo") || strings.Contains(categories.Body.String(), "estudos") {
		t.Fatal(categories.Body.String())
	}
	expect(req("owner", "DELETE", base+"/categories", `{"path":"livros_fisicos/referencia"}`), 200)
	another := req("owner", "POST", base+"/books", `{"title":"Outra edição","kind":"digital","format":"EPUB"}`)
	expect(another, 201)
	var edition Book
	json.Unmarshal(another.Body.Bytes(), &edition)
	relationship := fmt.Sprintf(`{"book_id":%q,"related_book_id":%q,"kind":"sample_of"}`, book.ID, edition.ID)
	expect(req("owner", "POST", base+"/relations", relationship), 200)
	links := req("owner", "GET", base+"/relations", "")
	expect(links, 200)
	if !strings.Contains(links.Body.String(), "sample_of") {
		t.Fatal(links.Body.String())
	}
	expect(req("other", "POST", base+"/relations", relationship), 404)
	expect(req("owner", "DELETE", base+"/relations", relationship), 200)

	expect(req("owner", "GET", "/api/catalogs/"+other+"/books", ""), 404) // Even root has no private-catalog bypass.
	expect(req("owner", "POST", base+"/members", `{"email":"guest@example.com"}`), 201)
	expect(req("guest", "GET", base+"/books", ""), 200)
	cached := req("guest", "GET", base+"/books", "")
	cachedRequest := httptest.NewRequest("GET", base+"/books", nil)
	cachedRequest.Header.Set("Authorization", "Bearer guest")
	cachedRequest.Header.Set("If-None-Match", cached.Header().Get("ETag"))
	cachedResponse := httptest.NewRecorder()
	handler.ServeHTTP(cachedResponse, cachedRequest)
	expect(cachedResponse, 304)

	expect(req("guest", "POST", base+"/members", `{"email":"third@example.com"}`), 403)
	expect(req("guest", "POST", base+"/books/"+book.ID+"/comments", `{"body":"Minha anotação","author_id":"owner"}`), 400)
	expect(req("guest", "POST", base+"/books/"+book.ID+"/comments", `{"body":"Minha anotação"}`), 201)
	w := req("owner", "GET", base+"/books/"+book.ID+"/comments", "")
	expect(w, 200)
	if !strings.Contains(w.Body.String(), `"email":"guest@example.com"`) {
		t.Fatal("wrong author: " + w.Body.String())
	}
	book.Favorite = false
	payload, _ := json.Marshal(book)
	expect(req("guest", "PUT", base+"/books/"+book.ID, string(payload)), 200)
	expect(req("owner", "PUT", base+"/books/"+book.ID, string(payload)), 409)
	expect(req("owner", "DELETE", base+"/members/guest@example.com", ""), 204)
	expect(req("guest", "GET", base+"/books", ""), 404)
	deniedCache := httptest.NewRecorder()
	handler.ServeHTTP(deniedCache, cachedRequest)
	expect(deniedCache, 404)

	expect(req("owner", "PUT", "/api/admin/access", `{"email":"owner@example.com","enabled":false,"admin":false}`), 409)
	expect(req("owner", "PUT", "/api/admin/access", `{"email":"next@example.com","enabled":true,"admin":true}`), 200)
	expect(req("owner", "PUT", "/api/admin/access", `{"email":"owner@example.com","enabled":true,"admin":false}`), 200)
	w = req("owner", "GET", "/api/me", "")
	expect(w, 200)
	if strings.Contains(w.Body.String(), `"admin":true`) {
		t.Fatal("bootstrap restored revoked admin")
	}
	expect(req("next", "PUT", "/api/admin/access", `{"email":"owner@example.com","enabled":true,"universal":false}`), 200)
	pending := 0
	if err = pool.QueryRow(context.Background(), `select count(*) from catalog.email_outbox`).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if pending != 1 {
		t.Fatalf("missing invite queue: %d", pending)
	}
	// Revoked invite is discarded, never delivered.
	if worked, err := s.DeliverNext(context.Background()); err != nil || worked {
		t.Fatalf("revoked invitation delivery: %v %v", worked, err)
	}
	expect(req("owner", "POST", base+"/members", `{"email":"delivery@example.com"}`), 201)
	if worked, err := s.DeliverNext(context.Background()); err != nil || !worked {
		t.Fatalf("invitation not delivered: %v %v", worked, err)
	}
	expect(req("owner", "DELETE", base+"/members/delivery@example.com", ""), 204)
	expect(req("owner", "GET", "/api/privacy/export", ""), 200)
	expect(req("owner", "POST", "/api/privacy/erasure", `{}`), 202)
	expect(req("other", "GET", "/api/admin/privacy", ""), 403)
	expect(req("next", "GET", "/api/admin/privacy", ""), 200)
	// 49 existing users + concurrent requests must never exceed 50.
	_, err = pool.Exec(context.Background(), `insert into catalog.members(catalog_id,email,created_by) select $1,'member'||n||'@example.com','owner' from generate_series(1,49) n`, id)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	codes := make(chan int, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			codes <- req("owner", "POST", base+"/members", fmt.Sprintf(`{"email":"race%d@example.com"}`, i)).Code
		}(i)
	}
	wg.Wait()
	close(codes)
	success := 0
	for code := range codes {
		if code == 201 {
			success++
		} else if code != 409 {
			t.Fatalf("unexpected code %d", code)
		}
	}
	if success != 1 {
		t.Fatalf("cap bypass: %d successes", success)
	}
	expect(req("next", "PUT", "/api/admin/access", `{"email":"owner@example.com","enabled":true,"universal":true}`), 200)
	expect(req("owner", "POST", base+"/members", `{"email":"unlimited@example.com"}`), 201)
	expect(req("next", "PUT", "/api/admin/access", `{"email":"owner@example.com","enabled":false,"universal":true}`), 200)
	expect(req("owner", "GET", base+"/books", ""), 403)
	expect(req("unlimited", "GET", base+"/books", ""), 403) // Suspended owner disables shared cloud access too.

	// Erasure is explicit, preserves a shared annotation, and retries provider failure safely.
	expect(req("next", "PUT", "/api/admin/access", `{"email":"owner@example.com","enabled":true}`), 200)
	otherBase := "/api/catalogs/" + other
	foreignBookResponse := req("other", "POST", otherBase+"/books", `{"title":"Acervo de outra pessoa","kind":"digital"}`)
	expect(foreignBookResponse, 201)
	var foreignBook Book
	json.Unmarshal(foreignBookResponse.Body.Bytes(), &foreignBook)
	expect(req("other", "POST", otherBase+"/members", `{"email":"owner@example.com"}`), 201)
	expect(req("owner", "POST", otherBase+"/books/"+foreignBook.ID+"/comments", `{"body":"Preservar o histórico compartilhado"}`), 201)
	var requestID string
	if err = pool.QueryRow(context.Background(), `select id::text from catalog.privacy_requests where user_id='owner'`).Scan(&requestID); err != nil {
		t.Fatal(err)
	}
	erasurePath := "/api/admin/privacy/" + requestID + "/complete"
	expect(req("other", "POST", erasurePath, `{"confirmation":"EXCLUIR"}`), 403)
	expect(req("next", "POST", erasurePath, `{"confirmation":""}`), 400)
	expect(req("next", "POST", erasurePath, `{"confirmation":"EXCLUIR"}`), 502)
	expect(req("owner", "GET", "/api/me", ""), 403)
	var count int
	if err = pool.QueryRow(context.Background(), `select count(*)from catalog.catalogs where owner_id='owner'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("owned catalog survived: %d %v", count, err)
	}
	remaining := req("other", "GET", otherBase+"/books/"+foreignBook.ID+"/comments", "")
	expect(remaining, 200)
	if !strings.Contains(remaining.Body.String(), "Conta excluída") || !strings.Contains(remaining.Body.String(), "Preservar o histórico compartilhado") || strings.Contains(remaining.Body.String(), "owner@example.com") {
		t.Fatal(remaining.Body.String())
	}
	expect(req("next", "POST", erasurePath, `{"confirmation":"EXCLUIR"}`), 200)
	expect(req("next", "POST", erasurePath, `{"confirmation":"EXCLUIR"}`), 200)
	if deleteCalls != 2 {
		t.Fatalf("provider retries are not idempotent: %d", deleteCalls)
	}
	expect(req("next", "POST", "/api/privacy/erasure", `{}`), 202)
	pool.QueryRow(context.Background(), `select id::text from catalog.privacy_requests where user_id='next'`).Scan(&requestID)
	expect(req("next", "POST", "/api/admin/privacy/"+requestID+"/complete", `{"confirmation":"EXCLUIR"}`), 409)
	// Private schema must not be exposed through Supabase's public roles.
	var permitted bool
	err = pool.QueryRow(context.Background(), `select has_schema_privilege('anon','catalog','USAGE')`).Scan(&permitted)
	if err != nil || permitted {
		t.Fatalf("schema exposed: %v %v", permitted, err)
	}
}
func TestEmailValidation(t *testing.T) {
	for _, value := range []string{"", "Name <a@example.com>", "bad"} {
		if _, err := normalizedEmail(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
}

var _ http.Handler = (*http.ServeMux)(nil)

type testMailer struct{}

func (testMailer) Send(context.Context, Message) error { return nil }
