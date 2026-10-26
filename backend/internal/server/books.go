package server

import (
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"net/http"
	"net/url"
	"strings"
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
	// No cloud reader or progress tracking; the reading flag remains a shelf filter.
	return nil
}
func (s *Server) books(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, false); err != nil {
		return err
	}
	var etag string
	if err := s.DB.QueryRow(r.Context(), `select md5(coalesce(string_agg(id::text||':'||version::text,',' order by id),'')) from catalog.books where catalog_id=$1 and deleted_at is null`, r.PathValue("catalog")).Scan(&etag); err != nil {
		return err
	}
	etag = "\"" + etag + "\""
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(304)
		return nil
	}
	rows, err := s.DB.Query(r.Context(), `select `+bookColumns+` from catalog.books where catalog_id=$1 and deleted_at is null order by lower(title),id`, r.PathValue("catalog"))
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
	raw, _ := json.Marshal(b.Metadata)
	b, err := readBook(s.DB.QueryRow(r.Context(), `insert into catalog.books(catalog_id,title,kind,format,path,filename,category,metadata,summary,cover_url,favorite,reading,updated_by) values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13) returning `+bookColumns, r.PathValue("catalog"), b.Title, b.Kind, b.Format, b.Path, b.Filename, b.Category, raw, b.Summary, b.CoverURL, b.Favorite, b.Reading, who(r).ID))
	if err != nil {
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
	raw, _ := json.Marshal(b.Metadata)
	b, err := readBook(s.DB.QueryRow(r.Context(), `update catalog.books set title=$3,kind=$4,format=$5,path=$6,filename=$7,category=$8,metadata=$9,summary=$10,cover_url=$11,favorite=$12,reading=$13,updated_by=$14,version=version+1,updated_at=now() where catalog_id=$1 and id=$2 and version=$15 and deleted_at is null returning `+bookColumns, r.PathValue("catalog"), r.PathValue("book"), b.Title, b.Kind, b.Format, b.Path, b.Filename, b.Category, raw, b.Summary, b.CoverURL, b.Favorite, b.Reading, who(r).ID, b.Version))
	if errors.Is(err, pgx.ErrNoRows) {
		return problem(409, "O livro foi alterado por outro usuário. Recarregue antes de salvar.")
	}
	if err != nil {
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
	result, err := s.DB.Exec(r.Context(), `update catalog.books set deleted_at=now(),updated_at=now(),version=version+1,updated_by=$3 where catalog_id=$1 and id=$2 and version=$4 and deleted_at is null`, r.PathValue("catalog"), r.PathValue("book"), who(r).ID, body.Version)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return problem(409, "O livro mudou ou já foi excluído.")
	}
	w.WriteHeader(204)
	return nil
}
func (s *Server) comments(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, false); err != nil {
		return err
	}
	rows, err := s.DB.Query(r.Context(), `select n.id::text,n.body,u.id,u.name,u.email,n.created_at::text from catalog.comments n join catalog.users u on u.id=n.author_id where n.catalog_id=$1 and n.book_id=$2 order by n.created_at,n.id`, r.PathValue("catalog"), r.PathValue("book"))
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, body, uid, name, email, at string
		if err = rows.Scan(&id, &body, &uid, &name, &email, &at); err != nil {
			return err
		}
		out = append(out, map[string]any{"id": id, "body": body, "author": Identity{uid, email, name}, "created_at": at})
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
	var id string
	err := s.DB.QueryRow(r.Context(), `insert into catalog.comments(catalog_id,book_id,author_id,body) select catalog_id,id,$3,$4 from catalog.books where catalog_id=$1 and id=$2 and deleted_at is null returning id::text`, r.PathValue("catalog"), r.PathValue("book"), who(r).ID, body.Body).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return problem(404, "Livro não encontrado.")
	}
	if err != nil {
		return err
	}
	respond(w, 201, map[string]string{"id": id})
	return nil
}
