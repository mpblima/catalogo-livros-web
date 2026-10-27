package server

import (
	"context"
	"encoding/json"
	"html"
	"net/http"
)

type AdminCatalog struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	OwnerEmail string   `json:"owner_email,omitempty"`
	Members    []string `json:"members,omitempty"`
}

type AdminUser struct {
	AdminIdentity
	Enabled        bool           `json:"enabled"`
	Universal      bool           `json:"universal"`
	Admin          bool           `json:"admin"`
	AccessGranted  bool           `json:"access_granted"`
	OwnedCatalogs  []AdminCatalog `json:"owned_catalogs"`
	SharedCatalogs []AdminCatalog `json:"shared_catalogs"`
}

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) error {
	actor := who(r)
	if !actor.Admin {
		return problem(403, "Acesso administrativo necessário.")
	}
	var active bool
	if err := s.DB.QueryRow(r.Context(), `select exists(select 1 from catalog.access_grants where email=$1 and admin and enabled)`, actor.Email).Scan(&active); err != nil {
		return err
	}
	if !active {
		return problem(403, "Permissão administrativa revogada.")
	}
	identities := []AdminIdentity{}
	if s.ListIdentities != nil {
		var err error
		identities, err = s.ListIdentities(r.Context())
		if err != nil {
			return problem(502, "Não foi possível consultar as contas do provedor de autenticação.")
		}
	}
	type stored struct {
		Email, Name               string
		Enabled, Universal, Admin bool
		AccessGranted             bool
		Owned, Shared             []AdminCatalog
	}
	known := map[string]stored{}
	rows, err := s.DB.Query(r.Context(), `select u.id,u.email,u.name,
coalesce(g.enabled,$1),coalesce(g.universal,false),coalesce(g.admin,false),(g.email is not null),
coalesce((select jsonb_agg(jsonb_build_object('id',c.id,'name',c.name,'members',coalesce((select jsonb_agg(m.email order by m.email) from catalog.members m where m.catalog_id=c.id),'[]'::jsonb)) order by c.created_at) from catalog.catalogs c where c.owner_id=u.id),'[]'::jsonb),
coalesce((select jsonb_agg(jsonb_build_object('id',c.id,'name',c.name,'owner_email',owner.email) order by c.created_at) from catalog.members m join catalog.catalogs c on c.id=m.catalog_id join catalog.users owner on owner.id=c.owner_id where m.email=u.email),'[]'::jsonb)
from catalog.users u left join catalog.access_grants g on g.email=u.email order by u.created_at desc`, s.Config.AutoActivate)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id, email, name string
		var item stored
		var owned, shared []byte
		if err = rows.Scan(&id, &email, &name, &item.Enabled, &item.Universal, &item.Admin, &item.AccessGranted, &owned, &shared); err != nil {
			rows.Close()
			return err
		}
		item.Email, item.Name = email, name
		if err = json.Unmarshal(owned, &item.Owned); err != nil {
			rows.Close()
			return err
		}
		if err = json.Unmarshal(shared, &item.Shared); err != nil {
			rows.Close()
			return err
		}
		known[id] = item
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if s.ListIdentities == nil {
		for id, item := range known {
			identities = append(identities, AdminIdentity{ID: id, Email: item.Email, Name: item.Name})
		}
	}
	out := make([]AdminUser, 0, len(identities))
	for _, identity := range identities {
		if identity.ID == "" || identity.Email == "" {
			continue
		}
		var erased bool
		if err = s.DB.QueryRow(r.Context(), `select exists(select 1 from catalog.erased_accounts where user_id=$1)`, identity.ID).Scan(&erased); err != nil {
			return err
		}
		if erased {
			continue
		}
		item, exists := known[identity.ID]
		user := AdminUser{AdminIdentity: identity, Enabled: s.Config.AutoActivate, OwnedCatalogs: []AdminCatalog{}, SharedCatalogs: []AdminCatalog{}}
		if exists {
			user.Email, user.Name = item.Email, item.Name
			user.Enabled, user.Universal, user.Admin, user.AccessGranted = item.Enabled, item.Universal, item.Admin, item.AccessGranted
			user.OwnedCatalogs, user.SharedCatalogs = item.Owned, item.Shared
		}
		if identity.Banned || identity.Locked {
			user.Enabled = false
		}
		out = append(out, user)
	}
	respond(w, 200, out)
	return nil
}

func (s *Server) requestAdminErasure(w http.ResponseWriter, r *http.Request) error {
	actor := who(r)
	if !actor.Admin {
		return problem(403, "Acesso administrativo necessário.")
	}
	if actor.ID == r.PathValue("user") {
		return problem(409, "Entre com outro administrador para excluir sua própria conta.")
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
	var active bool
	if err := s.DB.QueryRow(r.Context(), `select exists(select 1 from catalog.access_grants where email=$1 and admin and enabled)`, actor.Email).Scan(&active); err != nil {
		return err
	}
	if !active {
		return problem(403, "Permissão administrativa revogada.")
	}
	var targetEmail, targetName string
	if s.ListIdentities != nil {
		identities, err := s.ListIdentities(r.Context())
		if err != nil {
			return problem(502, "Não foi possível consultar as contas do provedor de autenticação.")
		}
		found := false
		for _, identity := range identities {
			if identity.ID == r.PathValue("user") {
				targetEmail, targetName, found = identity.Email, identity.Name, true
				break
			}
		}
		if !found {
			return problem(404, "Conta não encontrada no provedor de autenticação.")
		}
	} else if err := s.DB.QueryRow(r.Context(), `select email,name from catalog.users where id=$1`, r.PathValue("user")).Scan(&targetEmail, &targetName); err != nil {
		return problem(404, "Conta não encontrada.")
	}
	targetEmail, err := normalizedEmail(targetEmail)
	if err != nil {
		return problem(409, "A conta não tem um e-mail válido para remoção administrativa.")
	}
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	if _, err = tx.Exec(r.Context(), `insert into catalog.users(id,email,name) values($1,$2,$3) on conflict(id) do update set email=excluded.email,name=excluded.name`, r.PathValue("user"), targetEmail, targetName); err != nil {
		return err
	}
	var requestID string
	if err = tx.QueryRow(r.Context(), `insert into catalog.privacy_requests(user_id,kind) values($1,'erasure') on conflict(user_id,kind) do update set status=case when catalog.privacy_requests.status='completed' then 'pending' else catalog.privacy_requests.status end returning id::text`, r.PathValue("user")).Scan(&requestID); err != nil {
		return err
	}
	if _, err = tx.Exec(r.Context(), `insert into catalog.audit_events(actor_id,action,resource_id) values($1,'account.erase.request',$2)`, actor.ID, targetEmail); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	respond(w, 202, map[string]string{"request_id": requestID})
	return nil
}

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
