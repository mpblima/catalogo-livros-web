package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"image/png"
	"net/http"
	"strings"
)

type syncChange struct {
	LocalID     string           `json:"local_id"`
	BaseVersion int64            `json:"base_version"`
	Deleted     bool             `json:"deleted"`
	Restore     bool             `json:"restore,omitempty"`
	Book        Book             `json:"book"`
	Cover       string           `json:"cover_png,omitempty"`
	Annotations []syncAnnotation `json:"annotations,omitempty"`
}
type syncAnnotation struct {
	LocalID string `json:"local_id"`
	Body    string `json:"body"`
}
type syncRequest struct {
	Pull              *bool          `json:"pull,omitempty"`
	RequestID         string         `json:"request_id"`
	Cursor            int64          `json:"cursor"`
	Changes           []syncChange   `json:"changes"`
	ConfirmDeletions  bool           `json:"confirm_deletions"`
	Categories        []string       `json:"categories,omitempty"`
	CategoryDeletions []string       `json:"category_deletions,omitempty"`
	Relations         []syncRelation `json:"relations,omitempty"`
}
type syncRelation struct {
	Deleted bool   `json:"deleted,omitempty"`
	Book    string `json:"local_id"`
	Related string `json:"related_local_id"`
	Kind    string `json:"kind"`
}
type syncAck struct {
	LocalID string `json:"local_id"`
	BookID  string `json:"book_id"`
	Version int64  `json:"version"`
	Deleted bool   `json:"deleted"`
}
type syncConflict struct {
	LocalID string `json:"local_id"`
	Remote  Book   `json:"remote"`
	Deleted bool   `json:"deleted"`
	Cover   string `json:"cover_png,omitempty"`
}
type syncReceipt struct {
	Accepted  []syncAck      `json:"accepted"`
	Conflicts []syncConflict `json:"conflicts"`
}
type syncEvent struct {
	Seq     int64          `json:"seq"`
	Kind    string         `json:"kind"`
	Book    *Book          `json:"book,omitempty"`
	Deleted bool           `json:"deleted"`
	Cover   string         `json:"cover_png,omitempty"`
	Comment map[string]any `json:"comment,omitempty"`
}

// Desktop catalogs can contain legacy ISBN notes and page metadata that
// predate cloud validation. Preserve these values during sync; ISBN lookup and
// duplicate matching still validate ISBNs before using them.
func validateSyncedBook(b *Book) error {
	if b.Metadata == nil {
		b.Metadata = map[string]any{}
	}
	if strings.TrimSpace(b.Title) == "" {
		fallback := strings.TrimSpace(b.Filename)
		if fallback == "" || len(fallback) > 500 {
			fallback = "Livro sem título"
		}
		b.Title = fallback
	}
	rawISBN, hasISBN := b.Metadata["isbn"]
	invalidISBN := false
	if hasISBN {
		if normalized, err := isbnCode(textValue(rawISBN)); err == nil {
			b.Metadata["isbn"] = normalized
		} else {
			delete(b.Metadata, "isbn")
			invalidISBN = true
		}
	}
	legacyPages := map[string]any{}
	for _, field := range []string{"pages", "page"} {
		if value, ok := b.Metadata[field]; ok {
			legacyPages[field] = value
			delete(b.Metadata, field)
		}
	}
	err := validateBook(b)
	if invalidISBN {
		b.Metadata["isbn"] = rawISBN
	}
	for field, value := range legacyPages {
		b.Metadata[field] = value
	}
	return err
}

func syncCover(raw string) ([]byte, error) {
	if raw == "" {
		return nil, nil
	}
	if len(raw) > 710000 {
		return nil, problem(400, "Capa excede 512 KiB.")
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(data) > 524288 {
		return nil, problem(400, "Capa PNG inválida.")
	}
	info, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || info.Width < 1 || info.Height < 1 || info.Width > 2000 || info.Height > 2000 {
		return nil, problem(400, "Capa PNG inválida ou grande demais.")
	}
	if _, err = png.Decode(bytes.NewReader(data)); err != nil {
		return nil, problem(400, "Capa PNG incompleta.")
	}
	return data, nil
}
func (s *Server) syncCatalog(w http.ResponseWriter, r *http.Request) error {
	if who(r).DeviceID == "" || who(r).DeviceCatalog != r.PathValue("catalog") {
		return problem(403, "Use um desktop associado a este catálogo.")
	}
	if err := s.access(r, true); err != nil {
		return err
	}
	var body syncRequest
	if err := decode(w, r, &body); err != nil {
		return err
	}
	if _, err := uuid.Parse(body.RequestID); err != nil {
		return problem(400, "Lote sem identificador válido.")
	}
	if len(body.Changes) > 100 || len(body.Relations) > 500 || len(body.Categories) > 5000 || len(body.CategoryDeletions) > 5000 || body.Cursor < 0 {
		return problem(400, "Lote excede os limites da sincronização.")
	}
	raw, _ := json.Marshal(body)
	hash := sha256.Sum256(raw)
	payloadHash := hex.EncodeToString(hash[:])
	ctx := r.Context()
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	catalog, device := r.PathValue("catalog"), who(r).DeviceID
	if _, err = tx.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1,0))`, catalog); err != nil {
		return err
	}
	var savedHash string
	var saved []byte
	err = tx.QueryRow(ctx, `select payload_hash,response from catalog.sync_batches where device_id=$1 and request_id=$2`, device, body.RequestID).Scan(&savedHash, &saved)
	receipt := syncReceipt{Accepted: []syncAck{}, Conflicts: []syncConflict{}}
	if err == nil {
		if savedHash != payloadHash {
			return problem(409, "O identificador do lote já foi usado com outro conteúdo.")
		}
		if err = json.Unmarshal(saved, &receipt); err != nil {
			return err
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	} else {
		seen := map[string]bool{}
		for _, change := range body.Changes {
			if change.LocalID == "" || len(change.LocalID) > 200 || seen[change.LocalID] || change.BaseVersion < 0 {
				return problem(400, "Identificador local ou versão inválida.")
			}
			seen[change.LocalID] = true
			if change.Deleted && !body.ConfirmDeletions {
				return problem(400, "Exclusões precisam de confirmação explícita.")
			}
			var bookID string
			var base int64
			err = tx.QueryRow(ctx, `select book_id::text,base_version from catalog.sync_links where catalog_id=$1 and device_id=$2 and local_id=$3`, catalog, device, change.LocalID).Scan(&bookID, &base)
			exists := err == nil
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if !exists && change.BaseVersion != 0 {
				if _, err = uuid.Parse(change.Book.ID); err != nil {
					return problem(409, "Associação local ausente. Refaça a leitura do catálogo.")
				}
				var present bool
				if err = tx.QueryRow(ctx, `select exists(select 1 from catalog.books where catalog_id=$1 and id=$2)`, catalog, change.Book.ID).Scan(&present); err != nil {
					return err
				}
				if !present {
					return problem(404, "Livro não encontrado neste catálogo.")
				}
				bookID = change.Book.ID
				exists = true
			}
			if !exists && change.Deleted {
				continue
			}
			var old Book
			var deleted bool
			if exists {
				old, err = readBook(tx.QueryRow(ctx, `select `+bookColumns+` from catalog.books where catalog_id=$1 and id=$2 for update`, catalog, bookID))
				if err != nil {
					return err
				}
				if err = tx.QueryRow(ctx, `select deleted_at is not null from catalog.books where catalog_id=$1 and id=$2`, catalog, bookID).Scan(&deleted); err != nil {
					return err
				}
				if old.Version != change.BaseVersion || deleted && !change.Deleted && !change.Restore {
					conflict, e := readSyncConflict(ctx, tx, catalog, change.LocalID, old, deleted)
					if e != nil {
						return e
					}
					receipt.Conflicts = append(receipt.Conflicts, conflict)
					continue
				}
			}
			b := change.Book
			if b.Metadata == nil {
				b.Metadata = map[string]any{}
			}
			delete(b.Metadata, "page")
			if b.Kind == "digital" {
				delete(b.Metadata, "location")
			}
			if exists {
				for _, key := range []string{"page"} {
					if value, ok := old.Metadata[key]; ok {
						b.Metadata[key] = value
					}
				}
				if b.Kind == "digital" {
					if value, ok := old.Metadata["location"]; ok {
						b.Metadata["location"] = value
					}
				}
			}
			cover, err := syncCover(change.Cover)
			if err != nil {
				return err
			}
			if len(cover) > 0 {
				b.Metadata["_stored_cover"] = true
			} else if exists && old.Metadata["_stored_cover"] == true {
				b.Metadata["_stored_cover"] = true
			}
			if !change.Deleted {
				if err = validateSyncedBook(&b); err != nil {
					return err
				}
			}
			if !change.Deleted {
				match, e := findPhysicalMatch(ctx, tx, catalog, b, bookID)
				if e != nil {
					return e
				}
				if match != nil {
					if !exists {
						conflict, e := readSyncConflict(ctx, tx, catalog, change.LocalID, *match, false)
						if e != nil {
							return e
						}
						receipt.Conflicts = append(receipt.Conflicts, conflict)
						continue
					}
					return problem(409, "Este ISBN já pertence a outro cadastro físico. Revise o ISBN no desktop.")
				}
			}
			note, err := prepareBookNotes(ctx, tx, catalog, bookID, &b, who(r))
			if err != nil {
				return err
			}
			metadata, _ := json.Marshal(b.Metadata)
			if !exists {
				b, err = readBook(tx.QueryRow(ctx, `insert into catalog.books(catalog_id,title,kind,format,path,filename,category,metadata,summary,cover_url,favorite,reading,updated_by)values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)returning `+bookColumns, catalog, b.Title, b.Kind, b.Format, b.Path, b.Filename, b.Category, metadata, b.Summary, b.CoverURL, b.Favorite, b.Reading, who(r).ID))
				bookID = b.ID
			} else if change.Deleted {
				b, err = readBook(tx.QueryRow(ctx, `update catalog.books set deleted_at=coalesce(deleted_at,now()),updated_at=now(),version=version+1,updated_by=$3 where catalog_id=$1 and id=$2 returning `+bookColumns, catalog, bookID, who(r).ID))
			} else {
				b, err = readBook(tx.QueryRow(ctx, `update catalog.books set deleted_at=null,title=$3,kind=$4,format=$5,path=$6,filename=$7,category=$8,metadata=$9,summary=$10,cover_url=$11,favorite=$12,reading=$13,version=version+1,updated_at=now(),updated_by=$14 where catalog_id=$1 and id=$2 returning `+bookColumns, catalog, bookID, b.Title, b.Kind, b.Format, b.Path, b.Filename, b.Category, metadata, b.Summary, b.CoverURL, b.Favorite, b.Reading, who(r).ID))
			}
			if err != nil {
				return err
			}
			if len(cover) > 0 {
				if _, err = tx.Exec(ctx, `insert into catalog.book_covers(catalog_id,book_id,content)values($1,$2,$3)on conflict(book_id)do update set content=excluded.content`, catalog, bookID, cover); err != nil {
					return err
				}
			}
			if _, err = tx.Exec(ctx, `insert into catalog.sync_links(catalog_id,device_id,local_id,book_id,base_version,payload_hash)values($1,$2,$3,$4,$5,$6)on conflict(catalog_id,device_id,local_id)do update set base_version=excluded.base_version,payload_hash=excluded.payload_hash`, catalog, device, change.LocalID, bookID, b.Version, payloadHash); err != nil {
				return err
			}
			if len(change.Annotations) > 20 {
				return problem(400, "Anotações demais no mesmo registro.")
			}
			if !change.Deleted {
				if err = recordBookNotes(ctx, tx, catalog, bookID, who(r).ID, note, "desktop"); err != nil {
					return err
				}
			}
			for _, note := range change.Annotations {
				if note.LocalID == "" || len(note.LocalID) > 200 || strings.TrimSpace(note.Body) == "" || len(note.Body) > 20000 {
					return problem(400, "Anotação inválida.")
				}
				var present bool
				if err = tx.QueryRow(ctx, `select exists(select 1 from catalog.sync_annotations where catalog_id=$1 and device_id=$2 and local_id=$3)`, catalog, device, note.LocalID).Scan(&present); err != nil {
					return err
				}
				if present {
					continue
				}
				var commentID string
				if err = tx.QueryRow(ctx, `insert into catalog.comments(catalog_id,book_id,author_id,body,origin)values($1,$2,$3,$4,'desktop')returning id::text`, catalog, bookID, who(r).ID, note.Body).Scan(&commentID); err != nil {
					return err
				}
				if _, err = tx.Exec(ctx, `insert into catalog.sync_annotations(catalog_id,device_id,local_id,comment_id)values($1,$2,$3,$4)`, catalog, device, note.LocalID, commentID); err != nil {
					return err
				}
			}
			receipt.Accepted = append(receipt.Accepted, syncAck{change.LocalID, bookID, b.Version, change.Deleted})
		}
		for _, path := range body.Categories {
			if len(path) > 1000 || strings.TrimSpace(path) == "" {
				return problem(400, "Categoria inválida.")
			}
			if _, err = tx.Exec(ctx, `insert into catalog.categories(catalog_id,path)values($1,$2)on conflict do nothing`, catalog, path); err != nil {
				return err
			}
		}
		for _, path := range body.CategoryDeletions {
			if path == "livros_digitais" || path == "livros_fisicos" {
				continue
			}
			if _, err = tx.Exec(ctx, `delete from catalog.categories c where catalog_id=$1 and path=$2 and not exists(select 1 from catalog.books b where b.catalog_id=$1 and deleted_at is null and (category=$2 or starts_with(category,$2||'/'))) and not exists(select 1 from catalog.categories child where child.catalog_id=$1 and starts_with(child.path,$2||'/'))`, catalog, path); err != nil {
				return err
			}
		}
		for _, link := range body.Relations {
			if link.Kind != "same_work" && link.Kind != "sample_of" || link.Book == link.Related {
				return problem(400, "Relação inválida.")
			}
			if link.Deleted {
				if _, err = tx.Exec(ctx, `delete from catalog.book_relations r using catalog.sync_links a,catalog.sync_links b where r.catalog_id=$1 and a.catalog_id=$1 and b.catalog_id=$1 and a.device_id=$2 and b.device_id=$2 and a.local_id=$3 and b.local_id=$4 and r.book_id=a.book_id and r.related_book_id=b.book_id and r.kind=$5`, catalog, device, link.Book, link.Related, link.Kind); err != nil {
					return err
				}
				continue
			}
			_, err = tx.Exec(ctx, `insert into catalog.book_relations(catalog_id,book_id,related_book_id,kind,source)select $1,a.book_id,b.book_id,$5,'desktop'from catalog.sync_links a join catalog.sync_links b on b.catalog_id=a.catalog_id and b.device_id=a.device_id where a.catalog_id=$1 and a.device_id=$2 and a.local_id=$3 and b.local_id=$4 and a.book_id<>b.book_id on conflict do nothing`, catalog, device, link.Book, link.Related, link.Kind)
			if err != nil {
				return err
			}
		}
		saved, _ = json.Marshal(receipt)
		if _, err = tx.Exec(ctx, `insert into catalog.sync_batches(device_id,request_id,payload_hash,response)values($1,$2,$3,$4)`, device, body.RequestID, payloadHash, saved); err != nil {
			return err
		}
	}
	events := []syncEvent{}
	pullLimit := 101
	if body.Pull != nil && !*body.Pull {
		pullLimit = 0
	}
	rows, err := tx.Query(ctx, `select seq,kind,book_id::text,coalesce(comment_id::text,'')from catalog.sync_events where catalog_id=$1 and seq>$2 order by seq limit $3`, catalog, body.Cursor, pullLimit)
	if err != nil {
		return err
	}
	type eventKey struct {
		Seq                 int64
		Kind, Book, Comment string
	}
	keys := []eventKey{}
	for rows.Next() {
		var key eventKey
		if err = rows.Scan(&key.Seq, &key.Kind, &key.Book, &key.Comment); err != nil {
			rows.Close()
			return err
		}
		keys = append(keys, key)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	more := len(keys) > 100
	if more {
		keys = keys[:100]
	}
	cursor := body.Cursor
	eventBytes := 0
	for _, key := range keys {
		event := syncEvent{Seq: key.Seq, Kind: key.Kind}
		if key.Kind == "book" {
			b, err := readBook(tx.QueryRow(ctx, `select `+bookColumns+` from catalog.books where catalog_id=$1 and id=$2`, catalog, key.Book))
			if err != nil {
				return err
			}
			event.Book = &b
			if err = tx.QueryRow(ctx, `select deleted_at is not null from catalog.books where id=$1`, key.Book).Scan(&event.Deleted); err != nil {
				return err
			}
			var cover []byte
			err = tx.QueryRow(ctx, `select content from catalog.book_covers where book_id=$1 and catalog_id=$2`, key.Book, catalog).Scan(&cover)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			if len(cover) > 0 {
				event.Cover = base64.StdEncoding.EncodeToString(cover)
			}
		} else {
			var body, author, email, at, origin string
			if err = tx.QueryRow(ctx, `select c.body,u.name,u.email,c.created_at::text,c.origin from catalog.comments c join catalog.users u on u.id=c.author_id where c.id=$1 and c.catalog_id=$2`, key.Comment, catalog).Scan(&body, &author, &email, &at, &origin); err != nil {
				return err
			}
			event.Comment = map[string]any{"id": key.Comment, "book_id": key.Book, "body": body, "author_name": author, "author_email": email, "created_at": at, "origin": origin}
		}
		encoded, _ := json.Marshal(event)
		if eventBytes > 0 && eventBytes+len(encoded) > 1500000 {
			more = true
			break
		}
		eventBytes += len(encoded)
		events = append(events, event)
		cursor = key.Seq
	}
	paths := []string{}
	categoryRows, err := tx.Query(ctx, `select path from catalog.categories where catalog_id=$1 union select category from catalog.books where catalog_id=$1 and deleted_at is null and category<>'' order by 1`, catalog)
	if err != nil {
		return err
	}
	for categoryRows.Next() {
		var path string
		if err = categoryRows.Scan(&path); err != nil {
			categoryRows.Close()
			return err
		}
		paths = append(paths, path)
	}
	categoryRows.Close()
	if err = categoryRows.Err(); err != nil {
		return err
	}
	links := []relation{}
	linkRows, err := tx.Query(ctx, `select book_id::text,related_book_id::text,kind,source from catalog.book_relations where catalog_id=$1`, catalog)
	if err != nil {
		return err
	}
	for linkRows.Next() {
		var link relation
		if err = linkRows.Scan(&link.Book, &link.Related, &link.Kind, &link.Source); err != nil {
			linkRows.Close()
			return err
		}
		links = append(links, link)
	}
	linkRows.Close()
	if err = linkRows.Err(); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	respond(w, 200, map[string]any{"accepted": receipt.Accepted, "conflicts": receipt.Conflicts, "events": events, "cursor": cursor, "has_more": more, "categories": paths, "relations": links})
	return nil
}
func (s *Server) bookCover(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, false); err != nil {
		return err
	}
	var data []byte
	var version int64
	err := s.DB.QueryRow(r.Context(), `select c.content,b.version from catalog.book_covers c join catalog.books b on b.id=c.book_id where c.catalog_id=$1 and c.book_id=$2 and b.deleted_at is null`, r.PathValue("catalog"), r.PathValue("book")).Scan(&data, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return problem(404, "Capa não encontrada.")
	}
	if err != nil {
		return err
	}
	etag := fmt.Sprintf(`"cover-%d"`, version)
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "private, max-age=86400, immutable")
	w.Header().Add("Vary", "Authorization")
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return nil
	}
	w.Header().Set("Content-Type", "image/png")
	w.Write(data)
	return nil
}

func readSyncConflict(ctx context.Context, tx pgx.Tx, catalog, local string, book Book, deleted bool) (syncConflict, error) {
	conflict := syncConflict{LocalID: local, Remote: book, Deleted: deleted}
	var cover []byte
	err := tx.QueryRow(ctx, `select content from catalog.book_covers where catalog_id=$1 and book_id=$2`, catalog, book.ID).Scan(&cover)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return conflict, err
	}
	if len(cover) > 0 {
		conflict.Cover = base64.StdEncoding.EncodeToString(cover)
	}
	return conflict, nil
}
