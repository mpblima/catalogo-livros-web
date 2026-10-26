package server

import (
	"github.com/google/uuid"
	"golang.org/x/text/unicode/norm"
	"net/http"
	"strings"
	"unicode"
)

type relation struct {
	Book    string `json:"book_id"`
	Related string `json:"related_book_id"`
	Kind    string `json:"kind"`
	Source  string `json:"source,omitempty"`
}

func categoryPath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if len(raw) > 1000 || !strings.HasPrefix(raw, "livros_fisicos/") {
		return "", problem(400, "Use uma categoria dentro de Livros físicos.")
	}
	parts := strings.Split(raw, "/")
	if len(parts) > 16 {
		return "", problem(400, "Categoria com níveis demais.")
	}
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
		if parts[i] == "" || parts[i] == "." || parts[i] == ".." {
			return "", problem(400, "Nome de categoria inválido.")
		}
	}
	return strings.Join(parts, "/"), nil
}
func (s *Server) categories(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, false); err != nil {
		return err
	}
	rows, err := s.DB.Query(r.Context(), `select path from catalog.categories where catalog_id=$1 union select category from catalog.books where catalog_id=$1 and deleted_at is null and category<>'' order by 1`, r.PathValue("catalog"))
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var path string
		if err = rows.Scan(&path); err != nil {
			return err
		}
		out = append(out, path)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	respond(w, 200, out)
	return nil
}
func (s *Server) changeCategory(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, false); err != nil {
		return err
	}
	var body struct {
		Path    string `json:"path"`
		NewPath string `json:"new_path"`
	}
	if err := decode(w, r, &body); err != nil {
		return err
	}
	path, err := categoryPath(body.Path)
	if err != nil {
		return err
	}
	tx, err := s.beginCatalogWrite(r)
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	var id string
	if err = tx.QueryRow(r.Context(), `select id::text from catalog.catalogs where id=$1 for update`, r.PathValue("catalog")).Scan(&id); err != nil {
		return err
	}
	switch r.Method {
	case "POST":
		_, err = tx.Exec(r.Context(), `insert into catalog.categories(catalog_id,path)values($1,$2)on conflict do nothing`, id, path)
	case "PUT":
		newPath, e := categoryPath(body.NewPath)
		if e != nil {
			return e
		}
		if strings.HasPrefix(newPath, path+"/") {
			return problem(400, "Uma categoria não pode ser movida para dentro dela mesma.")
		}
		_, err = tx.Exec(r.Context(), `insert into catalog.categories(catalog_id,path)values($1,$2)on conflict do nothing`, id, path)
		if err == nil {
			_, err = tx.Exec(r.Context(), `insert into catalog.categories(catalog_id,path)select catalog_id,$3||substring(path from length($2)+1)from catalog.categories where catalog_id=$1 and(path=$2 or left(path,length($2)+1)=$2||'/')on conflict do nothing`, id, path, newPath)
		}
		if err == nil {
			_, err = tx.Exec(r.Context(), `update catalog.books set category=$3||substring(category from length($2)+1),version=version+1,updated_at=now(),updated_by=$4 where catalog_id=$1 and kind='physical' and deleted_at is null and(category=$2 or left(category,length($2)+1)=$2||'/')`, id, path, newPath, who(r).ID)
		}
		if err == nil && newPath != path {
			_, err = tx.Exec(r.Context(), `delete from catalog.categories where catalog_id=$1 and(path=$2 or left(path,length($2)+1)=$2||'/')`, id, path)
		}
	case "DELETE":
		var used bool
		err = tx.QueryRow(r.Context(), `select exists(select 1 from catalog.books where catalog_id=$1 and deleted_at is null and(category=$2 or left(category,length($2)+1)=$2||'/'))`, id, path).Scan(&used)
		if err != nil {
			return err
		}
		if used {
			return problem(409, "Mova os livros antes de excluir esta categoria.")
		}
		_, err = tx.Exec(r.Context(), `delete from catalog.categories where catalog_id=$1 and(path=$2 or left(path,length($2)+1)=$2||'/')`, id, path)
	}
	if err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	respond(w, 200, map[string]string{"path": path})
	return nil
}
func compactTitle(title string) string {
	var b strings.Builder
	for _, c := range norm.NFKD.String(strings.ToLower(title)) {
		if c >= 'a' && c <= 'z' || unicode.IsDigit(c) {
			b.WriteRune(c)
		}
	}
	return b.String()
}
func (s *Server) relations(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, false); err != nil {
		return err
	}
	rows, err := s.DB.Query(r.Context(), `select a.book_id::text,a.related_book_id::text,a.kind,a.source from catalog.book_relations a join catalog.books b on b.id=a.book_id join catalog.books c on c.id=a.related_book_id where a.catalog_id=$1 and b.deleted_at is null and c.deleted_at is null order by a.book_id,a.related_book_id,a.kind`, r.PathValue("catalog"))
	if err != nil {
		return err
	}
	out := []relation{}
	for rows.Next() {
		var a relation
		if err = rows.Scan(&a.Book, &a.Related, &a.Kind, &a.Source); err != nil {
			rows.Close()
			return err
		}
		out = append(out, a)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	books, err := s.DB.Query(r.Context(), `select id::text,title,metadata from catalog.books where catalog_id=$1 and kind='digital' and deleted_at is null order by id`, r.PathValue("catalog"))
	if err != nil {
		return err
	}
	defer books.Close()
	groups := map[string]string{}
	for books.Next() {
		var id, title string
		var meta map[string]any
		if err = books.Scan(&id, &title, &meta); err != nil {
			return err
		}
		isbn, e := isbnCode(textValue(meta["isbn"]))
		if e != nil {
			continue
		}
		key := isbn + "/" + compactTitle(title)
		if first := groups[key]; first != "" {
			out = append(out, relation{first, id, "same_work", "isbn_e_titulo"})
		} else {
			groups[key] = id
		}
	}
	if err = books.Err(); err != nil {
		return err
	}
	respond(w, 200, out)
	return nil
}
func (s *Server) changeRelation(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, false); err != nil {
		return err
	}
	var body relation
	if err := decode(w, r, &body); err != nil {
		return err
	}
	if body.Book == body.Related || body.Kind != "same_work" && body.Kind != "sample_of" {
		return problem(400, "Relação inválida.")
	}
	for _, id := range []string{body.Book, body.Related} {
		if _, err := uuid.Parse(id); err != nil {
			return problem(400, "Livro inválido.")
		}
	}
	var count int
	if err := s.DB.QueryRow(r.Context(), `select count(*)from catalog.books where catalog_id=$1 and id in($2,$3)and kind='digital' and deleted_at is null`, r.PathValue("catalog"), body.Book, body.Related).Scan(&count); err != nil {
		return err
	}
	if count != 2 {
		return problem(404, "Escolha dois livros digitais deste catálogo.")
	}
	if body.Kind == "same_work" && body.Book > body.Related {
		body.Book, body.Related = body.Related, body.Book
	}
	var err error
	if r.Method == "DELETE" {
		_, err = s.DB.Exec(r.Context(), `delete from catalog.book_relations where catalog_id=$1 and book_id=$2 and related_book_id=$3 and kind=$4`, r.PathValue("catalog"), body.Book, body.Related, body.Kind)
	} else {
		_, err = s.DB.Exec(r.Context(), `insert into catalog.book_relations(catalog_id,book_id,related_book_id,kind)values($1,$2,$3,$4)on conflict do nothing`, r.PathValue("catalog"), body.Book, body.Related, body.Kind)
	}
	if err != nil {
		return err
	}
	respond(w, 200, body)
	return nil
}
