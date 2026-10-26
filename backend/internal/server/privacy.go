package server

import (
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
)

func (s *Server) export(w http.ResponseWriter, r *http.Request) error {
	a := who(r)
	var data []byte
	err := s.DB.QueryRow(r.Context(), `select jsonb_build_object(
 'account',(select to_jsonb(u) from catalog.users u where u.id=$1),
 'owned_catalogs',coalesce((select jsonb_agg(to_jsonb(c)) from catalog.catalogs c where c.owner_id=$1),'[]'::jsonb),
 'books',coalesce((select jsonb_agg(to_jsonb(b)) from catalog.books b join catalog.catalogs c on c.id=b.catalog_id where c.owner_id=$1),'[]'::jsonb),
 'annotations',coalesce((select jsonb_agg(to_jsonb(n)) from catalog.comments n where n.author_id=$1),'[]'::jsonb))`, a.ID).Scan(&data)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="meus-dados.json"`)
	_, err = w.Write(data)
	return err
}
func (s *Server) requestErasure(w http.ResponseWriter, r *http.Request) error {
	_, err := s.DB.Exec(r.Context(), `insert into catalog.privacy_requests(user_id,kind) values($1,'erasure') on conflict(user_id,kind) do nothing`, who(r).ID)
	if err != nil {
		return err
	}
	respond(w, 202, map[string]string{"status": "pending", "message": "Solicitação registrada para análise do administrador."})
	return nil
}
func (s *Server) privacyRequests(w http.ResponseWriter, r *http.Request) error {
	if !who(r).Admin {
		return problem(403, "Acesso administrativo necessário.")
	}
	rows, err := s.DB.Query(r.Context(), `select p.id::text,u.email,p.created_at::text,p.status from catalog.privacy_requests p join catalog.users u on u.id=p.user_id order by p.created_at`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var id, email, at, status string
		if err = rows.Scan(&id, &email, &at, &status); err != nil {
			return err
		}
		out = append(out, map[string]string{"id": id, "email": email, "created_at": at, "status": status})
	}
	if err = rows.Err(); err != nil {
		return err
	}
	respond(w, 200, out)
	return nil
}

// Erasure is executed only for a recorded request and an explicit admin confirmation.
// Database deletion and the provider call have separate durable phases for safe retries.
func (s *Server) completeErasure(w http.ResponseWriter, r *http.Request) error {
	if !who(r).Admin {
		return problem(403, "Acesso administrativo necessário.")
	}
	if s.DeleteIdentity == nil {
		return problem(503, "Exclusão no provedor de autenticação não configurada.")
	}
	if _, err := uuid.Parse(r.PathValue("request")); err != nil {
		return problem(400, "Solicitação inválida.")
	}
	var body struct {
		Confirmation string `json:"confirmation"`
	}
	if err := decode(w, r, &body); err != nil {
		return err
	}
	if body.Confirmation != "EXCLUIR" {
		return problem(400, "Confirme a exclusão digitando EXCLUIR.")
	}
	ctx := r.Context()
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var initialized bool
	if err = tx.QueryRow(ctx, `select initialized from catalog.service_settings where singleton for update`).Scan(&initialized); err != nil {
		return err
	}
	var admin bool
	if err = tx.QueryRow(ctx, `select exists(select 1 from catalog.access_grants where email=$1 and admin and enabled)`, who(r).Email).Scan(&admin); err != nil {
		return err
	}
	if !admin {
		return problem(403, "Permissão administrativa revogada.")
	}
	var target, status, email string
	if err = tx.QueryRow(ctx, `select p.user_id,p.status,u.email from catalog.privacy_requests p join catalog.users u on u.id=p.user_id where p.id=$1 for update of p`, r.PathValue("request")).Scan(&target, &status, &email); errors.Is(err, pgx.ErrNoRows) {
		return problem(404, "Solicitação não encontrada.")
	} else if err != nil {
		return err
	}
	if status == "completed" {
		respond(w, 200, map[string]string{"status": "completed"})
		return nil
	}
	if status != "provider_pending" {
		var last bool
		if err = tx.QueryRow(ctx, `select exists(select 1 from catalog.access_grants where email=$1 and admin and enabled)and(select count(*)from catalog.access_grants where admin and enabled)<=1`, email).Scan(&last); err != nil {
			return err
		}
		if last {
			return problem(409, "Adicione outro administrador ativo antes de excluir esta conta.")
		}
		if _, err = tx.Exec(ctx, `insert into catalog.erased_accounts(user_id)values($1)on conflict do nothing`, target); err != nil {
			return err
		}
		statements := []string{
			`update catalog.books set metadata=(metadata-'notes_author_id'-'notes_author_name'-'notes_author_email')||'{"notes_author_name":"Conta excluída"}'::jsonb,version=version+1,updated_at=now() where metadata->>'notes_author_id'=$1`,
			`delete from catalog.sync_annotations where catalog_id in(select id from catalog.catalogs where owner_id=$1)or device_id in(select id from catalog.sync_devices where user_id=$1)`,
			`delete from catalog.sync_links where catalog_id in(select id from catalog.catalogs where owner_id=$1)`,
			`delete from catalog.sync_pairings where user_id=$1 or catalog_id in(select id from catalog.catalogs where owner_id=$1)`,
			`delete from catalog.sync_batches where device_id in(select id from catalog.sync_devices where user_id=$1)`,
			`delete from catalog.sync_devices where user_id=$1 or catalog_id in(select id from catalog.catalogs where owner_id=$1)`,
			`delete from catalog.email_outbox where catalog_id in(select id from catalog.catalogs where owner_id=$1)or recipient=(select email from catalog.users where id=$1)`,
			`delete from catalog.comments where catalog_id in(select id from catalog.catalogs where owner_id=$1)`,
			`delete from catalog.sync_events where catalog_id in(select id from catalog.catalogs where owner_id=$1)`,
			`delete from catalog.book_relations where catalog_id in(select id from catalog.catalogs where owner_id=$1)`,
			`delete from catalog.book_covers where catalog_id in(select id from catalog.catalogs where owner_id=$1)`,
			`delete from catalog.books where catalog_id in(select id from catalog.catalogs where owner_id=$1)`,
			`delete from catalog.categories where catalog_id in(select id from catalog.catalogs where owner_id=$1)`,
			`delete from catalog.members where catalog_id in(select id from catalog.catalogs where owner_id=$1)or email=(select email from catalog.users where id=$1)`,
			`delete from catalog.catalogs where owner_id=$1`,
			`delete from catalog.access_grants where email=(select email from catalog.users where id=$1)`,
		}
		for _, statement := range statements {
			if _, err = tx.Exec(ctx, statement, target); err != nil {
				return err
			}
		}
		if _, err = tx.Exec(ctx, `update catalog.audit_events set resource_id='erased-account' where resource_id=$1`, email); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `update catalog.users set name='Conta excluída',email=$2 where id=$1`, target, "erased+"+strings.ReplaceAll(r.PathValue("request"), "-", "")+"@invalid.example"); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `update catalog.privacy_requests set status='provider_pending',handled_by=$2 where id=$1`, r.PathValue("request"), who(r).ID); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if err = s.DeleteIdentity(ctx, target); err != nil {
		return problem(502, "Dados removidos da aplicação. A exclusão no Clerk está pendente; repita esta operação para concluir.")
	}
	if _, err = s.DB.Exec(ctx, `update catalog.privacy_requests set status='completed',completed_at=now()where id=$1`, r.PathValue("request")); err != nil {
		return err
	}
	respond(w, 200, map[string]string{"status": "completed"})
	return nil
}
