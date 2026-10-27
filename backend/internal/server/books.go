package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"golang.org/x/text/unicode/norm"
)

type Book struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Kind     string         `json:"kind"`
	Format   string         `json:"format"`
	Path     string         `json:"path"`
	Filename string         `json:"filename"`
	Category string         `json:"category"`
	Metadata map[string]any `json:"metadata"`
	Summary  string         `json:"summary"`
	CoverURL string         `json:"cover_url"`
	Favorite bool           `json:"favorite"`
	Reading  bool           `json:"reading"`
	Version  int64          `json:"version"`
}

const bookColumns = `id::text,title,kind,format,path,filename,category,metadata,summary,cover_url,favorite,reading,version`

type bookPage struct {
	Items  []Book `json:"items"`
	Total  int    `json:"total"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
	Groups int    `json:"groups"`
	Counts struct {
		All       int `json:"all"`
		Favorites int `json:"favorites"`
		Reading   int `json:"reading"`
	} `json:"counts"`
}

func bookSearchTerms(raw string) ([]string, error) {
	if utf8.RuneCountInString(raw) > 200 {
		return nil, problem(400, "A busca pode ter até 200 caracteres.")
	}
	normalized := norm.NFD.String(strings.ToLower(strings.TrimSpace(raw)))
	var clean strings.Builder
	for _, r := range normalized {
		if !unicode.Is(unicode.Mn, r) {
			clean.WriteRune(r)
		}
	}
	return strings.Fields(clean.String()), nil
}

func readBook(row pgx.Row) (Book, error) {
	var b Book
	err := row.Scan(&b.ID, &b.Title, &b.Kind, &b.Format, &b.Path, &b.Filename, &b.Category, &b.Metadata, &b.Summary, &b.CoverURL, &b.Favorite, &b.Reading, &b.Version)
	return b, err
}
func validateBook(b *Book) error {
	b.Title = strings.TrimSpace(b.Title)
	if b.Title == "" || len(b.Title) > 500 {
		return problem(400, "Informe um título com até 500 caracteres.")
	}
	if b.Kind != "physical" && b.Kind != "digital" {
		return problem(400, "Tipo de livro inválido.")
	}
	if b.Metadata == nil {
		b.Metadata = map[string]any{}
	}
	if len(b.Path) > 4000 || len(b.Summary) > 100000 || len(b.Category) > 1000 {
		return problem(400, "Campos excedem o tamanho permitido.")
	}
	if b.CoverURL != "" {
		u, err := url.Parse(b.CoverURL)
		if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil {
			return problem(400, "A capa precisa de uma URL HTTPS válida.")
		}
	}
	for _, key := range []string{"pages", "page"} {
		if value, exists := b.Metadata[key]; exists && fmt.Sprint(value) != "" {
			raw := fmt.Sprint(value)
			if number, ok := value.(float64); ok {
				raw = strconv.FormatFloat(number, 'f', -1, 64)
			}
			n, err := strconv.Atoi(raw)
			limit := 1000000
			if key == "pages" {
				limit = 9999999
			}
			if err != nil || n < 1 || n > limit {
				return problem(400, "Número de páginas ou página atual inválida.")
			}
		}
	}
	if raw := textValue(b.Metadata["isbn"]); raw != "" {
		normalized, err := isbnCode(raw)
		if err != nil {
			return err
		}
		b.Metadata["isbn"] = normalized
	}
	return nil
}
func (s *Server) books(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, false); err != nil {
		return err
	}
	kind := r.URL.Query().Get("kind")
	if kind != "" && kind != "physical" && kind != "digital" {
		return problem(400, "Tipo de livro inválido.")
	}
	if _, paginated := r.URL.Query()["limit"]; paginated || r.URL.Query().Has("offset") {
		limit, offset := 30, 0
		var err error
		if raw := r.URL.Query().Get("limit"); raw != "" {
			limit, err = strconv.Atoi(raw)
			if err != nil || limit < 1 || limit > 100 {
				return problem(400, "O limite precisa ficar entre 1 e 100 livros.")
			}
		}
		if raw := r.URL.Query().Get("offset"); raw != "" {
			offset, err = strconv.Atoi(raw)
			if err != nil || offset < 0 || offset > 10000000 {
				return problem(400, "A página solicitada é inválida.")
			}
		}
		category := strings.TrimSpace(r.URL.Query().Get("category"))
		if len(category) > 1000 {
			return problem(400, "A categoria é inválida.")
		}
		shelf := r.URL.Query().Get("shelf")
		if shelf == "" {
			shelf = "all"
		}
		if shelf != "all" && shelf != "favorites" && shelf != "reading" {
			return problem(400, "A estante solicitada é inválida.")
		}
		terms, err := bookSearchTerms(r.URL.Query().Get("q"))
		if err != nil {
			return err
		}
		const filter = `catalog_id=$1 and deleted_at is null and ($2='' or kind=$2) and ($3='' or category=$3 or left(category,length($3)+1)=$3||'/') and ($4='all' or ($4='favorites' and favorite) or ($4='reading' and reading)) and not exists(select 1 from unnest($5::text[]) term where position(term in translate(lower(concat_ws(' ',title,category,path,filename,metadata::text,summary)),'áàâãäåéèêëíìîïóòôõöúùûüçñ','aaaaaaeeeeiiiiooooouuuucn'))=0)`
		const pageQuery = `with recursive filtered as (
 select b.* from catalog.books b where ` + filter + `
), automatic_groups as (
 select id,first_value(id) over(partition by upper(regexp_replace(metadata->>'isbn','[^0-9Xx]','','g')),regexp_replace(translate(lower(title),'áàâãäåéèêëíìîïóòôõöúùûüçñ','aaaaaaeeeeiiiiooooouuuucn'),'[^a-z0-9]','','g') order by id) anchor_id
 from filtered where coalesce(metadata->>'isbn','')<>''
), edges(a,b) as (
 select x.book_id,x.related_book_id from catalog.book_relations x join filtered f on f.id=x.book_id join filtered g on g.id=x.related_book_id where x.catalog_id=$1
 union select x.related_book_id,x.book_id from catalog.book_relations x join filtered f on f.id=x.book_id join filtered g on g.id=x.related_book_id where x.catalog_id=$1
 union select id,anchor_id from automatic_groups where id<>anchor_id
 union select anchor_id,id from automatic_groups where id<>anchor_id
), reach(start_id,node_id) as (
 select id,id from filtered
 union
 select reach.start_id,edges.b from reach join edges on edges.a=reach.node_id
), roots as (
 select start_id,min(node_id::text)::uuid root_id from reach group by start_id
), group_roots as (
 select r.root_id,min(lower(f.title)) sort_title from roots r join filtered f on f.id=r.start_id group by r.root_id
), page_roots as (
 select root_id from group_roots order by sort_title,root_id limit $7 offset $6
), members as (
 select r.start_id from roots r join page_roots p on p.root_id=r.root_id
)
select (select count(*) from filtered),(select count(*) from group_roots),coalesce((select array_agg(start_id::text) from members),'{}'::text[])`
		var total, groups int
		var ids []string
		if err = s.DB.QueryRow(r.Context(), pageQuery, r.PathValue("catalog"), kind, category, shelf, terms, offset, limit).Scan(&total, &groups, &ids); err != nil {
			return err
		}
		out := bookPage{Items: []Book{}, Total: total, Offset: offset, Limit: limit, Groups: groups}
		if err = s.DB.QueryRow(r.Context(), `select count(*),count(*) filter(where favorite),count(*) filter(where reading) from catalog.books where catalog_id=$1 and deleted_at is null and ($2='' or kind=$2)`, r.PathValue("catalog"), kind).Scan(&out.Counts.All, &out.Counts.Favorites, &out.Counts.Reading); err != nil {
			return err
		}
		rows, err := s.DB.Query(r.Context(), `select `+bookColumns+` from catalog.books where catalog_id=$1 and id=any($2::uuid[]) and deleted_at is null order by lower(title),id`, r.PathValue("catalog"), ids)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			b, err := readBook(rows)
			if err != nil {
				return err
			}
			out.Items = append(out.Items, b)
		}
		if err = rows.Err(); err != nil {
			return err
		}
		respond(w, 200, out)
		return nil
	}
	var etag string
	if err := s.DB.QueryRow(r.Context(), `select md5(coalesce(string_agg(id::text||':'||version::text,',' order by id),'')) from catalog.books where catalog_id=$1 and deleted_at is null and ($2='' or kind=$2)`, r.PathValue("catalog"), kind).Scan(&etag); err != nil {
		return err
	}
	etag = "\"" + etag + "\""
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(304)
		return nil
	}
	rows, err := s.DB.Query(r.Context(), `select `+bookColumns+` from catalog.books where catalog_id=$1 and deleted_at is null and ($2='' or kind=$2) order by lower(title),id`, r.PathValue("catalog"), kind)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []Book{}
	for rows.Next() {
		b, err := readBook(rows)
		if err != nil {
			return err
		}
		out = append(out, b)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	respond(w, 200, out)
	return nil
}
func (s *Server) createBook(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, false); err != nil {
		return err
	}
	var b Book
	if err := decode(w, r, &b); err != nil {
		return err
	}
	if err := validateBook(&b); err != nil {
		return err
	}
	tx, err := s.beginCatalogWrite(r)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if match, e := findPhysicalMatch(r.Context(), tx, r.PathValue("catalog"), b, ""); e != nil {
		return e
	} else if match != nil {
		return problem(409, "Este ISBN já está cadastrado em Livros físicos.")
	}
	note, err := prepareBookNotes(r.Context(), tx, r.PathValue("catalog"), "", &b, who(r))
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(b.Metadata)
	b, err = readBook(tx.QueryRow(r.Context(), `insert into catalog.books(catalog_id,title,kind,format,path,filename,category,metadata,summary,cover_url,favorite,reading,updated_by) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) returning `+bookColumns, r.PathValue("catalog"), b.Title, b.Kind, b.Format, b.Path, b.Filename, b.Category, raw, b.Summary, b.CoverURL, b.Favorite, b.Reading, who(r).ID))
	if err != nil {
		return err
	}
	if err = recordBookNotes(r.Context(), tx, r.PathValue("catalog"), b.ID, who(r).ID, note, "web"); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	respond(w, 201, b)
	return nil
}
func (s *Server) updateBook(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, false); err != nil {
		return err
	}
	var b Book
	if err := decode(w, r, &b); err != nil {
		return err
	}
	if err := validateBook(&b); err != nil {
		return err
	}
	if b.Version < 1 {
		return problem(400, "Versão do livro obrigatória.")
	}
	tx, err := s.beginCatalogWrite(r)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if match, e := findPhysicalMatch(r.Context(), tx, r.PathValue("catalog"), b, r.PathValue("book")); e != nil {
		return e
	} else if match != nil {
		return problem(409, "Este ISBN já está cadastrado em Livros físicos.")
	}
	note, err := prepareBookNotes(r.Context(), tx, r.PathValue("catalog"), r.PathValue("book"), &b, who(r))
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(b.Metadata)
	b, err = readBook(tx.QueryRow(r.Context(), `update catalog.books set title=$3,kind=$4,format=$5,path=$6,filename=$7,category=$8,metadata=$9,summary=$10,cover_url=$11,favorite=$12,reading=$13,updated_by=$14,version=version+1,updated_at=now() where catalog_id=$1 and id=$2 and version=$15 and deleted_at is null returning `+bookColumns, r.PathValue("catalog"), r.PathValue("book"), b.Title, b.Kind, b.Format, b.Path, b.Filename, b.Category, raw, b.Summary, b.CoverURL, b.Favorite, b.Reading, who(r).ID, b.Version))
	if errors.Is(err, pgx.ErrNoRows) {
		return problem(409, "O livro foi alterado por outro usuário. Recarregue antes de salvar.")
	}
	if err != nil {
		return err
	}
	if err = recordBookNotes(r.Context(), tx, r.PathValue("catalog"), b.ID, who(r).ID, note, "web"); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	respond(w, 200, b)
	return nil
}
func (s *Server) deleteBook(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, false); err != nil {
		return err
	}
	var body struct {
		Version int64 `json:"version"`
	}
	if err := decode(w, r, &body); err != nil {
		return err
	}
	tx, err := s.beginCatalogWrite(r)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	result, err := tx.Exec(r.Context(), `update catalog.books set deleted_at=now(),updated_at=now(),version=version+1,updated_by=$3 where catalog_id=$1 and id=$2 and version=$4 and deleted_at is null`, r.PathValue("catalog"), r.PathValue("book"), who(r).ID, body.Version)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return problem(409, "O livro mudou ou já foi excluído.")
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	w.WriteHeader(204)
	return nil
}
func (s *Server) comments(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, false); err != nil {
		return err
	}
	rows, err := s.DB.Query(r.Context(), `select n.id::text,n.body,u.id,u.name,u.email,n.created_at::text,n.origin from catalog.comments n join catalog.users u on u.id=n.author_id where n.catalog_id=$1 and n.book_id=$2 order by n.created_at,n.id`, r.PathValue("catalog"), r.PathValue("book"))
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, body, uid, name, email, at, origin string
		if err = rows.Scan(&id, &body, &uid, &name, &email, &at, &origin); err != nil {
			return err
		}
		out = append(out, map[string]any{"id": id, "body": body, "author": Identity{uid, email, name}, "created_at": at, "origin": origin})
	}
	if err = rows.Err(); err != nil {
		return err
	}
	respond(w, 200, out)
	return nil
}
func (s *Server) addComment(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, false); err != nil {
		return err
	}
	var body struct {
		Body string `json:"body"`
	}
	if err := decode(w, r, &body); err != nil {
		return err
	}
	body.Body = strings.TrimSpace(body.Body)
	if body.Body == "" || len(body.Body) > 20000 {
		return problem(400, "Escreva uma anotação com até 20.000 caracteres.")
	}
	tx, err := s.beginCatalogWrite(r)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	var id string
	err = tx.QueryRow(r.Context(), `insert into catalog.comments(catalog_id,book_id,author_id,body) select catalog_id,id,$3,$4 from catalog.books where catalog_id=$1 and id=$2 and deleted_at is null returning id::text`, r.PathValue("catalog"), r.PathValue("book"), who(r).ID, body.Body).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return problem(404, "Livro não encontrado.")
	}
	if err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	respond(w, 201, map[string]string{"id": id})
	return nil
}

func (s *Server) beginCatalogWrite(r *http.Request) (pgx.Tx, error) {
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		return nil, err
	}
	if _, err = tx.Exec(r.Context(), `select pg_advisory_xact_lock(hashtextextended($1,0))`, r.PathValue("catalog")); err != nil {
		tx.Rollback(r.Context())
		return nil, err
	}
	return tx, nil
}

func findPhysicalMatch(ctx context.Context, tx pgx.Tx, catalog string, b Book, ignore string) (*Book, error) {
	if b.Kind != "physical" {
		return nil, nil
	}
	isbn, err := isbnCode(textValue(b.Metadata["isbn"]))
	if err != nil {
		return nil, nil
	}
	rows, err := tx.Query(ctx, `select `+bookColumns+` from catalog.books where catalog_id=$1 and kind='physical' and deleted_at is null and($2=''or id::text<>$2)and coalesce(metadata->>'isbn','')<>''`, catalog, ignore)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		book, err := readBook(rows)
		if err != nil {
			return nil, err
		}
		candidate, err := isbnCode(textValue(book.Metadata["isbn"]))
		if err == nil && candidate == isbn {
			return &book, nil
		}
	}
	return nil, rows.Err()
}
