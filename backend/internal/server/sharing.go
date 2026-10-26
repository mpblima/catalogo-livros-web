package server

import (
	"context"
	"html"
	"net/http"
)

func (s *Server) members(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, true); err != nil {
		return err
	}
	rows, err := s.DB.Query(r.Context(), `select email from catalog.members where catalog_id=$1 order by email`, r.PathValue("catalog"))
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var email string
		if err = rows.Scan(&email); err != nil {
			return err
		}
		out = append(out, email)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	respond(w, 200, out)
	return nil
}
func (s *Server) addMember(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, true); err != nil {
		return err
	}
	if s.Mailer == nil || s.Config.FrontendURL == "" {
		return problem(503, "Envio de convites ainda não configurado.")
	}
	var body struct {
		Email string `json:"email"`
	}
	if err := decode(w, r, &body); err != nil {
		return err
	}
	email, err := normalizedEmail(body.Email)
	if err != nil {
		return err
	}
	a := who(r)
	if email == a.Email {
		return problem(400, "Você já é responsável pelo catálogo.")
	}
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	// Serialize additions per catalog so concurrent invitations cannot bypass the cap.
	var id string
	if err = tx.QueryRow(r.Context(), `select id::text from catalog.catalogs where id=$1 for update`, r.PathValue("catalog")).Scan(&id); err != nil {
		return err
	}
	var count int
	var exists bool
	if err = tx.QueryRow(r.Context(), `select count(*),coalesce(bool_or(email=$2),false) from catalog.members where catalog_id=$1`, id, email).Scan(&count, &exists); err != nil {
		return err
	}
	if !exists && !a.Universal && count >= 50 {
		return problem(409, "O catálogo já tem 50 usuários compartilhados.")
	}
	_, err = tx.Exec(r.Context(), `insert into catalog.members(catalog_id,email,created_by) values($1,$2,$3) on conflict do nothing`, id, email, a.ID)
	if err != nil {
		return err
	}
	if !exists {
		invite := "<p>Você recebeu um convite de " + html.EscapeString(a.Name) + " para compartilhar um catálogo de livros.</p><p>Crie sua conta ou entre usando o e-mail " + html.EscapeString(email) + ".</p><p><a href=\"" + html.EscapeString(s.Config.FrontendURL) + "\">Acessar o catálogo</a></p><p>Se não conhece quem enviou, ignore esta mensagem.</p>"
		if _, err = tx.Exec(r.Context(), `insert into catalog.email_outbox(catalog_id,recipient,subject,html) values($1,$2,$3,$4)`, id, email, "Convite para compartilhar um catálogo de livros", invite); err != nil {
			return err
		}
		if _, err = tx.Exec(r.Context(), `insert into catalog.audit_events(actor_id,action,resource_id) values($1,'member.add',$2)`, a.ID, id); err != nil {
			return err
		}
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	respond(w, 201, map[string]string{"email": email})
	return nil
}
func (s *Server) removeMember(w http.ResponseWriter, r *http.Request) error {
	if err := s.access(r, true); err != nil {
		return err
	}
	_, err := s.DB.Exec(r.Context(), `delete from catalog.members where catalog_id=$1 and email=$2`, r.PathValue("catalog"), r.PathValue("email"))
	if err != nil {
		return err
	}
	w.WriteHeader(204)
	return nil
}
func (s *Server) grants(w http.ResponseWriter, r *http.Request) error {
	if !who(r).Admin {
		return problem(403, "Acesso administrativo necessário.")
	}
	rows, err := s.DB.Query(r.Context(), `select email,enabled,universal,admin from catalog.access_grants order by email`)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var email string
		var enabled, universal, admin bool
		if err = rows.Scan(&email, &enabled, &universal, &admin); err != nil {
			return err
		}
		out = append(out, map[string]any{"email": email, "enabled": enabled, "universal": universal, "admin": admin})
	}
	if err = rows.Err(); err != nil {
		return err
	}
	respond(w, 200, out)
	return nil
}
func (s *Server) grant(w http.ResponseWriter, r *http.Request) error {
	a := who(r)
	if !a.Admin {
		return problem(403, "Acesso administrativo necessário.")
	}
	var b struct {
		Email     string `json:"email"`
		Enabled   bool   `json:"enabled"`
		Universal bool   `json:"universal"`
		Admin     bool   `json:"admin"`
	}
	if err := decode(w, r, &b); err != nil {
		return err
	}
	email, err := normalizedEmail(b.Email)
	if err != nil {
		return err
	}
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	// Also serializes bootstrap and concurrent last-admin changes.
	if _, err = tx.Exec(r.Context(), `select singleton from catalog.service_settings where singleton for update`); err != nil {
		return err
	}
	var stillAdmin bool
	if err = tx.QueryRow(r.Context(), `select exists(select 1 from catalog.access_grants where email=$1 and admin and enabled)`, a.Email).Scan(&stillAdmin); err != nil {
		return err
	}
	if !stillAdmin {
		return problem(403, "Permissão administrativa revogada.")
	}
	var count int
	var targetAdmin bool
	if err = tx.QueryRow(r.Context(), `select count(*),coalesce(bool_or(email=$1),false) from catalog.access_grants where admin and enabled`, email).Scan(&count, &targetAdmin); err != nil {
		return err
	}
	if targetAdmin && count <= 1 && (!b.Admin || !b.Enabled) {
		return problem(409, "Adicione outro administrador ativo antes de remover este acesso.")
	}
	_, err = tx.Exec(r.Context(), `insert into catalog.access_grants(email,enabled,universal,admin,granted_by) values($1,$2,$3,$4,$5) on conflict(email) do update set enabled=excluded.enabled,universal=excluded.universal,admin=excluded.admin,granted_by=excluded.granted_by,updated_at=now()`, email, b.Enabled, b.Universal || b.Admin, b.Admin, a.ID)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `insert into catalog.audit_events(actor_id,action,resource_id) values($1,'access.update',$2)`, a.ID, email); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	respond(w, 200, map[string]string{"email": email})
	return nil
}

func (s *Server) bootstrapAdmin(ctx context.Context, a account) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var initialized bool
	if err = tx.QueryRow(ctx, `select initialized from catalog.service_settings where singleton for update`).Scan(&initialized); err != nil {
		return err
	}
	if !initialized {
		if _, err = tx.Exec(ctx, `insert into catalog.access_grants(email,enabled,universal,admin,granted_by) values($1,true,true,true,$2) on conflict(email) do update set enabled=true,universal=true,admin=true`, a.Email, a.ID); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `update catalog.service_settings set initialized=true where singleton`); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
