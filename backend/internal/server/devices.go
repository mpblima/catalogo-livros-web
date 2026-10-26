package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"net"
	"net/http"
	"regexp"
	"strings"
)

func tokenHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func (s *Server) publicDevice(h handler) http.HandlerFunc {
	return route(func(w http.ResponseWriter, r *http.Request) error {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if !s.limiter.allow("pair:" + ip) {
			return problem(429, "Muitas tentativas. Aguarde um minuto.")
		}
		return h(w, r)
	})
}
func (s *Server) startPair(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		DeviceID  string `json:"device_id"`
		Name      string `json:"device_name"`
		Challenge string `json:"challenge"`
	}
	if err := decode(w, r, &body); err != nil {
		return err
	}
	if _, err := uuid.Parse(body.DeviceID); err != nil {
		return problem(400, "Dispositivo inválido.")
	}
	body.Name = strings.TrimSpace(body.Name)
	if body.Name == "" || len(body.Name) > 80 || !regexp.MustCompile(`^[a-f0-9]{64}$`).MatchString(body.Challenge) {
		return problem(400, "Nome ou desafio de conexão inválido.")
	}
	raw := make([]byte, 5)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	code := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	// Expired pairing records contain no reusable device tokens and can be discarded.
	if _, err := s.DB.Exec(r.Context(), `delete from catalog.sync_pairings where expires_at<now()`); err != nil {
		return err
	}
	var id string
	if err := s.DB.QueryRow(r.Context(), `insert into catalog.sync_pairings(device_id,device_name,challenge,user_code)values($1,$2,$3,$4)returning id::text`, body.DeviceID, body.Name, body.Challenge, code).Scan(&id); err != nil {
		return err
	}
	respond(w, 201, map[string]any{"id": id, "user_code": code, "verification_url": s.Config.FrontendURL + "/?conectar=" + id, "expires_in": 600})
	return nil
}
func (s *Server) pairInfo(w http.ResponseWriter, r *http.Request) error {
	if _, err := uuid.Parse(r.PathValue("pair")); err != nil {
		return problem(400, "Conexão inválida.")
	}
	var name string
	if err := s.DB.QueryRow(r.Context(), `select device_name from catalog.sync_pairings where id=$1 and expires_at>now() and consumed_at is null`, r.PathValue("pair")).Scan(&name); errors.Is(err, pgx.ErrNoRows) {
		return problem(404, "Esta conexão expirou ou já foi concluída.")
	} else if err != nil {
		return err
	}
	respond(w, 200, map[string]string{"device_name": name})
	return nil
}
func (s *Server) approvePair(w http.ResponseWriter, r *http.Request) error {
	if who(r).DeviceID != "" || !who(r).Enabled {
		return problem(403, "Entre pela interface web para conectar um desktop.")
	}
	var body struct {
		Code    string `json:"user_code"`
		Catalog string `json:"catalog_id"`
	}
	if err := decode(w, r, &body); err != nil {
		return err
	}
	if _, err := uuid.Parse(body.Catalog); err != nil {
		return problem(400, "Catálogo inválido.")
	}
	if _, err := uuid.Parse(r.PathValue("pair")); err != nil {
		return problem(400, "Conexão inválida.")
	}
	var owned bool
	if err := s.DB.QueryRow(r.Context(), `select exists(select 1 from catalog.catalogs where id=$1 and owner_id=$2)`, body.Catalog, who(r).ID).Scan(&owned); err != nil {
		return err
	}
	if !owned {
		return problem(403, "Conecte somente um catálogo de sua responsabilidade.")
	}
	tx, err := s.DB.Begin(r.Context())
	if err != nil {
		return err
	}
	defer tx.Rollback(r.Context())
	var code string
	var user, catalog *string
	if err = tx.QueryRow(r.Context(), `select user_code,user_id,catalog_id::text from catalog.sync_pairings where id=$1 and expires_at>now() and consumed_at is null for update`, r.PathValue("pair")).Scan(&code, &user, &catalog); errors.Is(err, pgx.ErrNoRows) {
		return problem(404, "Conexão expirada.")
	} else if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(strings.ToUpper(strings.TrimSpace(body.Code))), []byte(code)) != 1 {
		return problem(400, "O código não confere com o exibido no desktop.")
	}
	if user != nil && (*user != who(r).ID || catalog == nil || *catalog != body.Catalog) {
		return problem(409, "Esta conexão já foi aprovada para outra conta ou catálogo.")
	}
	if _, err = tx.Exec(r.Context(), `update catalog.sync_pairings set user_id=$2,catalog_id=$3,approved_at=now()where id=$1`, r.PathValue("pair"), who(r).ID, body.Catalog); err != nil {
		return err
	}
	if err = tx.Commit(r.Context()); err != nil {
		return err
	}
	respond(w, 200, map[string]string{"status": "approved"})
	return nil
}
func (s *Server) exchangePair(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Verifier string `json:"verifier"`
	}
	if err := decode(w, r, &body); err != nil {
		return err
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{43,128}$`).MatchString(body.Verifier) {
		return problem(400, "Verificador inválido.")
	}
	if _, err := uuid.Parse(r.PathValue("pair")); err != nil {
		return problem(400, "Conexão inválida.")
	}
	ctx := r.Context()
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var challenge, device, name string
	var user, catalog *string
	var consumed bool
	if err = tx.QueryRow(ctx, `select challenge,device_id::text,device_name,user_id,catalog_id::text,consumed_at is not null from catalog.sync_pairings where id=$1 and expires_at>now()for update`, r.PathValue("pair")).Scan(&challenge, &device, &name, &user, &catalog, &consumed); errors.Is(err, pgx.ErrNoRows) {
		return problem(410, "Conexão expirada.")
	} else if err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(challenge), []byte(tokenHash(body.Verifier))) != 1 {
		return problem(401, "Verificador incorreto.")
	}
	if user == nil || catalog == nil {
		respond(w, 202, map[string]string{"status": "pending"})
		return nil
	}
	var allowed bool
	if err = tx.QueryRow(ctx, `select not exists(select 1 from catalog.erased_accounts where user_id=$1)and coalesce((select enabled from catalog.access_grants where email=(select email from catalog.users where id=$1)),$2)`, *user, s.Config.AutoActivate).Scan(&allowed); err != nil {
		return err
	}
	if !allowed {
		return problem(403, "A conta não está ativa.")
	}
	token := "catd_" + body.Verifier
	if !consumed {
		tag, err := tx.Exec(ctx, `insert into catalog.sync_devices(id,user_id,catalog_id,name,token_hash,expires_at)values($1,$2,$3,$4,$5,now()+interval '30 days')on conflict(id)do update set token_hash=excluded.token_hash,name=excluded.name,expires_at=excluded.expires_at,revoked_at=null where catalog.sync_devices.user_id=excluded.user_id and catalog.sync_devices.catalog_id=excluded.catalog_id`, device, *user, *catalog, name, tokenHash(token))
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return problem(409, "Dispositivo já associado. Inicie uma nova conexão no desktop.")
		}
		if _, err = tx.Exec(ctx, `update catalog.sync_pairings set consumed_at=now()where id=$1`, r.PathValue("pair")); err != nil {
			return err
		}
	}
	var active bool
	if err = tx.QueryRow(ctx, `select exists(select 1 from catalog.sync_devices where id=$1 and token_hash=$2 and revoked_at is null and expires_at>now())`, device, tokenHash(token)).Scan(&active); err != nil {
		return err
	}
	if !active {
		return problem(410, "A conexão foi revogada. Conecte novamente.")
	}
	var email, catalogName string
	if err = tx.QueryRow(ctx, `select u.email,c.name from catalog.users u join catalog.catalogs c on c.owner_id=u.id where u.id=$1 and c.id=$2`, *user, *catalog).Scan(&email, &catalogName); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	respond(w, 200, map[string]string{"device_id": device, "catalog_id": *catalog, "catalog_name": catalogName, "email": email, "token": token})
	return nil
}
func (s *Server) deviceIdentity(ctx context.Context, token, path string) (Identity, string, string, error) {
	var uid, device, catalog string
	err := s.DB.QueryRow(ctx, `select user_id,id::text,catalog_id::text from catalog.sync_devices where token_hash=$1 and revoked_at is null and expires_at>now()`, tokenHash(token)).Scan(&uid, &device, &catalog)
	if err != nil || s.LookupIdentity == nil {
		return Identity{}, "", "", problem(401, "Conexão desktop inválida ou expirada.")
	}
	if path != "/api/catalogs/"+catalog+"/sync" && path != "/api/devices/"+device {
		return Identity{}, "", "", problem(403, "Este token só permite sincronizar o catálogo associado e desconectar o dispositivo.")
	}
	identity, err := s.LookupIdentity(ctx, uid)
	return identity, device, catalog, err
}
func (s *Server) devices(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.DB.Query(r.Context(), `select id::text,name,expires_at::text,revoked_at is not null from catalog.sync_devices where user_id=$1 order by created_at desc`, who(r).ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, name, expires string
		var revoked bool
		if err = rows.Scan(&id, &name, &expires, &revoked); err != nil {
			return err
		}
		out = append(out, map[string]any{"id": id, "name": name, "expires_at": expires, "revoked": revoked})
	}
	if err = rows.Err(); err != nil {
		return err
	}
	respond(w, 200, out)
	return nil
}
func (s *Server) revokeDevice(w http.ResponseWriter, r *http.Request) error {
	if _, err := uuid.Parse(r.PathValue("device")); err != nil {
		return problem(400, "Dispositivo inválido.")
	}
	tag, err := s.DB.Exec(r.Context(), `update catalog.sync_devices set revoked_at=now()where id=$1 and user_id=$2`, r.PathValue("device"), who(r).ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return problem(404, "Dispositivo não encontrado.")
	}
	respond(w, 200, map[string]string{"status": "revoked"})
	return nil
}
