package server

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (s *Server) loans(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, false); err != nil {
		return err
	}
	a := who(r)
	rows, err := s.DB.Query(r.Context(), `select l.id::text,l.book_id::text,b.title,b.kind,l.borrower_name,l.borrower_email,l.status,l.requested_at::text,l.decided_at::text,l.returned_at::text,c.owner_id=$2,l.note from catalog.book_loans l join catalog.books b on b.catalog_id=l.catalog_id and b.id=l.book_id join catalog.catalogs c on c.id=l.catalog_id where l.catalog_id=$1 and (c.owner_id=$2 or l.borrower_id=$2) order by l.requested_at desc`, r.PathValue("catalog"), a.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, bookID, title, kind, name, email, status, requested, note string
		var decided, returned *string
		var owner bool
		if err = rows.Scan(&id, &bookID, &title, &kind, &name, &email, &status, &requested, &decided, &returned, &owner, &note); err != nil {
			return err
		}
		out = append(out, map[string]any{"id": id, "book_id": bookID, "title": title, "kind": kind, "borrower_name": name, "borrower_email": email, "status": status, "requested_at": requested, "decided_at": decided, "returned_at": returned, "owner": owner, "note": note})
	}
	if err = rows.Err(); err != nil {
		return err
	}
	respond(w, http.StatusOK, out)
	return nil
}

func (s *Server) requestLoan(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, false); err != nil {
		return err
	}
	a := who(r)
	if _, err := uuid.Parse(r.PathValue("book")); err != nil {
		return problem(400, "Identificador de livro inválido.")
	}
	var body struct {
		Note           string `json:"note"`
		RecipientName  string `json:"recipient_name"`
		RecipientEmail string `json:"recipient_email"`
	}
	if err := decode(w, r, &body); err != nil {
		return err
	}
	if len(strings.TrimSpace(body.Note)) > 1000 {
		return problem(400, "A mensagem pode ter até 1.000 caracteres.")
	}
	var owner bool
	var kind, title string
	err := s.DB.QueryRow(r.Context(), `select c.owner_id=$3,b.kind,b.title from catalog.books b join catalog.catalogs c on c.id=b.catalog_id where b.catalog_id=$1 and b.id=$2 and b.deleted_at is null`, r.PathValue("catalog"), r.PathValue("book"), a.ID).Scan(&owner, &kind, &title)
	if err == pgx.ErrNoRows {
		return problem(404, "Livro não encontrado.")
	}
	if err != nil {
		return err
	}
	if owner {
		name := strings.TrimSpace(body.RecipientName)
		email, e := normalizedEmail(body.RecipientEmail)
		if name == "" || len(name) > 200 || e != nil {
			return problem(400, "Informe nome e e-mail válidos de quem receberá o livro.")
		}
		status := "active"
		if kind == "digital" {
			status = "sent"
		}
		if kind == "physical" {
			var occupied bool
			if err = s.DB.QueryRow(r.Context(), `select exists(select 1 from catalog.book_loans where book_id=$1 and status='active')`, r.PathValue("book")).Scan(&occupied); err != nil {
				return err
			}
			if occupied {
				return problem(409, "Este exemplar já está emprestado. Registre a devolução antes de emprestá-lo novamente.")
			}
		}
		var id string
		err = s.DB.QueryRow(r.Context(), `insert into catalog.book_loans(catalog_id,book_id,borrower_id,borrower_name,borrower_email,note,status,decided_at) values($1,$2,(select id from catalog.users where email=$4),$3,$4,$5,$6,now()) returning id::text`, r.PathValue("catalog"), r.PathValue("book"), name, email, strings.TrimSpace(body.Note), status).Scan(&id)
		if err != nil {
			return err
		}
		respond(w, http.StatusCreated, map[string]any{"id": id, "title": title, "kind": kind, "status": status})
		return nil
	}
	var id string
	err = s.DB.QueryRow(r.Context(), `insert into catalog.book_loans(catalog_id,book_id,borrower_id,borrower_name,borrower_email,note,status) values($1,$2,$3,$4,$5,$6,'requested') returning id::text`, r.PathValue("catalog"), r.PathValue("book"), a.ID, a.Name, a.Email, strings.TrimSpace(body.Note)).Scan(&id)
	if err != nil {
		return err
	}
	respond(w, http.StatusCreated, map[string]any{"id": id, "title": title, "kind": kind, "status": "requested"})
	return nil
}

func (s *Server) loanAction(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, true); err != nil {
		return err
	}
	if _, err := uuid.Parse(r.PathValue("loan")); err != nil {
		return problem(400, "Identificador de solicitação inválido.")
	}
	var body struct {
		Action string `json:"action"`
	}
	if err := decode(w, r, &body); err != nil {
		return err
	}
	a := who(r)
	var status, kind string
	err := s.DB.QueryRow(r.Context(), `select l.status,b.kind from catalog.book_loans l join catalog.books b on b.catalog_id=l.catalog_id and b.id=l.book_id where l.catalog_id=$1 and l.id=$2`, r.PathValue("catalog"), r.PathValue("loan")).Scan(&status, &kind)
	if err == pgx.ErrNoRows {
		return problem(404, "Solicitação não encontrada.")
	}
	if err != nil {
		return err
	}
	var next string
	switch body.Action {
	case "approve":
		if kind != "physical" || status != "requested" {
			return problem(409, "Somente um pedido físico pendente pode ser aprovado.")
		}
		next = "active"
	case "send":
		if kind != "digital" || status != "requested" {
			return problem(409, "Somente um pedido digital pendente pode ser marcado como enviado.")
		}
		next = "sent"
	case "reject":
		if status != "requested" {
			return problem(409, "Somente pedidos pendentes podem ser recusados.")
		}
		next = "rejected"
	case "return":
		if kind != "physical" || status != "active" {
			return problem(409, "Somente um empréstimo físico ativo pode ser devolvido.")
		}
		next = "returned"
	default:
		return problem(400, "Ação de empréstimo inválida.")
	}
	if next == "active" {
		var occupied bool
		if err = s.DB.QueryRow(r.Context(), `select exists(select 1 from catalog.book_loans where book_id=(select book_id from catalog.book_loans where catalog_id=$1 and id=$2) and status='active')`, r.PathValue("catalog"), r.PathValue("loan")).Scan(&occupied); err != nil {
			return err
		}
		if occupied {
			return problem(409, "Este exemplar já está emprestado.")
		}
	}
	result, err := s.DB.Exec(r.Context(), `update catalog.book_loans set status=$3,decided_at=case when status='requested' then now() else decided_at end,returned_at=case when $3='returned' then now() else returned_at end where catalog_id=$1 and id=$2 and status=$4`, r.PathValue("catalog"), r.PathValue("loan"), next, status)
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return problem(409, "O pedido mudou enquanto você processava a ação. Atualize a lista e tente novamente.")
	}
	_, _ = s.DB.Exec(r.Context(), `insert into catalog.audit_events(actor_id,action,resource_id) values($1,$2,$3)`, a.ID, "loan."+body.Action, r.PathValue("loan"))
	w.WriteHeader(http.StatusNoContent)
	return nil
}
