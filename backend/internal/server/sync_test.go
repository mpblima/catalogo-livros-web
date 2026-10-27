package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mpblima/catalogo-livros-web/backend/internal/migrate"
	"image"
	"image/png"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestValidateSyncedBookPreservesLegacyISBN(t *testing.T) {
	for _, isbn := range []string{"9781484245316; 9781484245293", "not-a-valid-isbn"} {
		book := Book{Title: "Legacy catalog entry", Kind: "physical", Metadata: map[string]any{"isbn": isbn, "pages": 0, "page": "legacy"}}
		if err := validateSyncedBook(&book); err != nil {
			t.Fatalf("sync rejected legacy ISBN %q: %v", isbn, err)
		}
		if got := book.Metadata["isbn"]; got != isbn {
			t.Fatalf("sync changed legacy ISBN: got %v, want %q", got, isbn)
		}
		if book.Metadata["pages"] != 0 || book.Metadata["page"] != "legacy" {
			t.Fatalf("sync changed legacy page metadata: %v", book.Metadata)
		}
	}
	book := Book{Title: "Valid ISBN", Kind: "physical", Metadata: map[string]any{"isbn": "0-13-468599-7"}}
	if err := validateSyncedBook(&book); err != nil || book.Metadata["isbn"] != "9780134685991" {
		t.Fatalf("valid ISBN was not normalized: metadata=%v err=%v", book.Metadata, err)
	}
	book = Book{Title: "", Kind: "physical", Metadata: map[string]any{"isbn": "legacy value", "pages": 0}}
	if err := validateSyncedBook(&book); err == nil {
		t.Fatal("sync accepted a book with an empty title")
	}
	if got := book.Metadata["isbn"]; got != "legacy value" {
		t.Fatalf("validation failure lost legacy ISBN: got %v", got)
	}
	if book.Metadata["pages"] != 0 {
		t.Fatalf("validation failure lost legacy page metadata: %v", book.Metadata)
	}
}

func TestDesktopSync(t *testing.T) {
	if os.Getenv("TEST_DATABASE_URL") == "" {
		t.Skip("disposable database required")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("sync_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "create database "+name); err != nil {
		t.Fatal(err)
	}
	cfg := admin.Config().Copy()
	cfg.Database = name
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = migrate.Run(ctx, conn, migrate.Options{Directory: "../../../supabase/migrations"}); err != nil {
		t.Fatal(err)
	}
	conn.Close(ctx)
	poolConfig, _ := pgxpool.ParseConfig(os.Getenv("TEST_DATABASE_URL"))
	poolConfig.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { pool.Close(); admin.Exec(ctx, "drop database "+name); admin.Close(ctx) }()
	identity := func(ctx context.Context, id string) (Identity, error) {
		return Identity{id, id + "@example.com", id}, nil
	}
	s := Server{DB: pool, Auth: identity, LookupIdentity: identity, Config: Config{BootstrapAdminEmail: "owner@example.com", AutoActivate: true, FrontendURL: "http://localhost:5173"}}
	h := s.Handler()
	req := func(token, method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var data []byte
		if body != nil {
			data, _ = json.Marshal(body)
		}
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	expect := func(w *httptest.ResponseRecorder, status int) {
		t.Helper()
		if w.Code != status {
			t.Fatalf("wanted %d got %d: %s", status, w.Code, w.Body.String())
		}
	}
	catalogs := req("owner", "GET", "/api/catalogs", nil)
	expect(catalogs, 200)
	var list []struct{ ID string }
	json.Unmarshal(catalogs.Body.Bytes(), &list)
	catalog := list[0].ID
	base := "/api/catalogs/" + catalog
	verifier := strings.Repeat("A", 43)
	device := uuid.NewString()
	start := req("", "POST", "/device/pair", map[string]string{"device_id": device, "device_name": "Test desktop", "challenge": tokenHash(verifier)})
	expect(start, 201)
	var pair map[string]any
	json.Unmarshal(start.Body.Bytes(), &pair)
	id := pair["id"].(string)
	code := pair["user_code"].(string)
	expect(req("", "POST", "/device/pair/"+id+"/token", map[string]string{"verifier": verifier}), 202)
	expect(req("other", "POST", "/api/devices/pair/"+id+"/approve", map[string]string{"catalog_id": catalog, "user_code": code}), 403)
	expect(req("owner", "POST", "/api/devices/pair/"+id+"/approve", map[string]string{"catalog_id": catalog, "user_code": "WRONG"}), 400)
	expect(req("owner", "POST", "/api/devices/pair/"+id+"/approve", map[string]string{"catalog_id": catalog, "user_code": code}), 200)
	expect(req("", "POST", "/device/pair/"+id+"/token", map[string]string{"verifier": strings.Repeat("B", 43)}), 401)
	exchange := req("", "POST", "/device/pair/"+id+"/token", map[string]string{"verifier": verifier})
	expect(exchange, 200)
	var credentials map[string]string
	json.Unmarshal(exchange.Body.Bytes(), &credentials)
	token := credentials["token"]
	expect(req(token, "GET", "/api/admin/access", nil), 403)
	expect(req("owner", "POST", base+"/sync", syncRequest{RequestID: uuid.NewString()}), 403)
	expect(req(token, "POST", "/api/catalogs/"+uuid.NewString()+"/sync", syncRequest{RequestID: uuid.NewString()}), 403)
	var cover bytes.Buffer
	png.Encode(&cover, image.NewRGBA(image.Rect(0, 0, 4, 4)))
	upload := syncRequest{RequestID: uuid.NewString(), Changes: []syncChange{{LocalID: "local-1", Book: Book{Title: "Livro do desktop", Kind: "digital", Format: "PDF", Path: "books/local.pdf", Metadata: map[string]any{"page": "99", "location": "pdf-scroll:99:0.4", "isbn": "0134685997"}}, Cover: base64.StdEncoding.EncodeToString(cover.Bytes()), Annotations: []syncAnnotation{{LocalID: "note1", Body: "Nota preservada do desktop"}}}}}
	result := req(token, "POST", base+"/sync", upload)
	expect(result, 200)
	var response struct {
		Accepted  []syncAck      `json:"accepted"`
		Conflicts []syncConflict `json:"conflicts"`
		Events    []syncEvent    `json:"events"`
		Cursor    int64          `json:"cursor"`
	}
	json.Unmarshal(result.Body.Bytes(), &response)
	if len(response.Accepted) != 1 || len(response.Events) != 2 {
		t.Fatal(result.Body.String())
	}
	cloudID := response.Accepted[0].BookID
	expect(req(token, "POST", base+"/sync", upload), 200)
	var count int
	pool.QueryRow(ctx, `select count(*)from catalog.books`).Scan(&count)
	if count != 1 {
		t.Fatal("replayed batch duplicated books")
	}
	pool.QueryRow(ctx, `select count(*)from catalog.comments`).Scan(&count)
	if count != 1 {
		t.Fatal("replayed batch duplicated notes")
	}
	altered := upload
	altered.Cursor = 2
	expect(req(token, "POST", base+"/sync", altered), 409)
	public := req("owner", "GET", base+"/books", nil)
	var books []Book
	json.Unmarshal(public.Body.Bytes(), &books)
	book := books[0]
	if book.Metadata["page"] != nil || book.Metadata["location"] != nil || book.Metadata["_stored_cover"] != true {
		t.Fatalf("private position leaked or cover missing: %#v", book)
	}
	coverResponse := req("owner", "GET", base+"/books/"+cloudID+"/cover", nil)
	expect(coverResponse, 200)
	if !bytes.Equal(coverResponse.Body.Bytes(), cover.Bytes()) {
		t.Fatal("cover did not round trip")
	}
	expect(req("other", "GET", base+"/books/"+cloudID+"/cover", nil), 404)
	book.Title = "Versão da nuvem"
	book.Metadata["page"] = "12"
	edited := req("owner", "PUT", base+"/books/"+cloudID, book)
	expect(edited, 200)
	json.Unmarshal(edited.Body.Bytes(), &book)
	conflict := upload
	conflict.RequestID = uuid.NewString()
	conflict.Changes[0].BaseVersion = 1
	conflict.Changes[0].Book.Title = "Versão local"
	result = req(token, "POST", base+"/sync", conflict)
	expect(result, 200)
	json.Unmarshal(result.Body.Bytes(), &response)
	if len(response.Conflicts) != 1 || response.Conflicts[0].Remote.Title != "Versão da nuvem" {
		t.Fatal(result.Body.String())
	}
	conflict.RequestID = uuid.NewString()
	conflict.Changes[0].BaseVersion = book.Version
	result = req(token, "POST", base+"/sync", conflict)
	expect(result, 200)
	json.Unmarshal(result.Body.Bytes(), &response)
	if len(response.Accepted) != 1 {
		t.Fatal(result.Body.String())
	}
	version := response.Accepted[0].Version
	pool.QueryRow(ctx, `select count(*)from catalog.comments`).Scan(&count)
	if count != 1 {
		t.Fatal("revised batch duplicated annotation")
	}
	deletion := syncRequest{RequestID: uuid.NewString(), Changes: []syncChange{{LocalID: "local-1", BaseVersion: version, Deleted: true}}}
	expect(req(token, "POST", base+"/sync", deletion), 400)
	deletion.ConfirmDeletions = true
	expect(req(token, "POST", base+"/sync", deletion), 200)
	expect(req("owner", "DELETE", "/api/devices/"+device, nil), 200)
	expect(req(token, "POST", base+"/sync", syncRequest{RequestID: uuid.NewString()}), 401)
	expect(req("", "POST", "/device/pair/"+id+"/token", map[string]string{"verifier": verifier}), 410)
}
