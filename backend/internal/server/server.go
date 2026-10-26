package server

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"log/slog"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

type Config struct {
	BootstrapAdminEmail string
	Origins             []string
	AutoActivate        bool
	FrontendURL         string
}
type Server struct {
	DB      *pgxpool.Pool
	Auth    Authenticator
	Config  Config
	Mailer  Mailer
	limiter *rateLimiter
}
type account struct {
	Identity
	Admin     bool `json:"admin"`
	Enabled   bool `json:"enabled"`
	Universal bool `json:"universal"`
}
type key struct{}
type handler func(http.ResponseWriter, *http.Request) error
type apiError struct {
	Code    int
	Message string
}

func (e apiError) Error() string         { return e.Message }
func problem(code int, msg string) error { return apiError{code, msg} }
func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
func route(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			var ae apiError
			if errors.As(err, &ae) {
				respond(w, ae.Code, map[string]string{"error": ae.Message})
			} else {
				slog.Error("request failed", "method", r.Method, "path", r.URL.Path, "error_type", fmtErrorType(err))
				respond(w, 500, map[string]string{"error": "Não foi possível concluir a operação."})
			}
		}
	}
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return problem(400, "Dados inválidos.")
	}
	if d.Decode(&struct{}{}) != io.EOF {
		return problem(400, "Apenas um objeto JSON é permitido.")
	}
	return nil
}
func who(r *http.Request) account { return r.Context().Value(key{}).(account) }
func (s *Server) Handler() http.Handler {
	s.limiter = &rateLimiter{buckets: map[string]bucket{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := s.DB.Ping(ctx); err != nil {
			respond(w, 503, map[string]string{"status": "unavailable"})
			return
		}
		respond(w, 200, map[string]string{"status": "ok"})
	})
	api := http.NewServeMux()
	api.HandleFunc("GET /api/me", route(s.me))
	api.HandleFunc("GET /api/books/lookup", route(s.lookupBook))
	api.HandleFunc("GET /api/privacy/export", route(s.export))
	api.HandleFunc("POST /api/privacy/erasure", route(s.requestErasure))
	api.HandleFunc("GET /api/admin/privacy", route(s.privacyRequests))
	api.HandleFunc("GET /api/catalogs", route(s.catalogs))
	api.HandleFunc("GET /api/catalogs/{catalog}/books", route(s.books))
	api.HandleFunc("POST /api/catalogs/{catalog}/books", route(s.createBook))
	api.HandleFunc("PUT /api/catalogs/{catalog}/books/{book}", route(s.updateBook))
	api.HandleFunc("DELETE /api/catalogs/{catalog}/books/{book}", route(s.deleteBook))
	api.HandleFunc("GET /api/catalogs/{catalog}/books/{book}/comments", route(s.comments))
	api.HandleFunc("POST /api/catalogs/{catalog}/books/{book}/comments", route(s.addComment))
	api.HandleFunc("GET /api/catalogs/{catalog}/members", route(s.members))
	api.HandleFunc("POST /api/catalogs/{catalog}/members", route(s.addMember))
	api.HandleFunc("DELETE /api/catalogs/{catalog}/members/{email}", route(s.removeMember))
	api.HandleFunc("GET /api/admin/access", route(s.grants))
	api.HandleFunc("PUT /api/admin/access", route(s.grant))
	mux.Handle("/api/", s.authenticate(api))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		origin := r.Header.Get("Origin")
		if origin != "" {
			allowed := false
			for _, o := range s.Config.Origins {
				if origin == o {
					allowed = true
				}
			}
			if !allowed {
				respond(w, 403, map[string]string{"error": "Origem não autorizada."})
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Expose-Headers", "ETag")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, If-None-Match")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
func (s *Server) authenticate(next http.Handler) http.Handler {
	return route(func(w http.ResponseWriter, r *http.Request) error {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" || token == r.Header.Get("Authorization") {
			return problem(401, "Entre na sua conta.")
		}
		identity, err := s.Auth(r.Context(), token)
		if err != nil {
			return problem(401, "Sessão inválida. Entre novamente.")
		}
		if !s.limiter.allow(identity.ID) {
			w.Header().Set("Retry-After", "60")
			return problem(429, "Muitas requisições. Tente novamente em um minuto.")
		}
		a := account{Identity: identity}
		_, err = s.DB.Exec(r.Context(), `insert into catalog.users(id,email,name) values($1,$2,$3) on conflict(id) do update set email=excluded.email,name=excluded.name`, a.ID, a.Email, a.Name)
		if err != nil {
			return err
		}
		if strings.EqualFold(a.Email, s.Config.BootstrapAdminEmail) {
			if err = s.bootstrapAdmin(r.Context(), a); err != nil {
				return err
			}
		}
		var enabled, universal, admin bool
		err = s.DB.QueryRow(r.Context(), `select enabled,universal,admin from catalog.access_grants where email=$1`, a.Email).Scan(&enabled, &universal, &admin)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		a.Admin = admin && enabled
		a.Enabled = enabled || (errors.Is(err, pgx.ErrNoRows) && s.Config.AutoActivate)
		a.Universal = a.Admin || universal
		if err == nil && !enabled {
			return problem(403, "Sua conta está suspensa. Contate um administrador.")
		}
		if value := r.PathValue("catalog"); value != "" {
			if _, err := uuid.Parse(value); err != nil {
				return problem(400, "Identificador inválido.")
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), key{}, a)))
		return nil
	})
}
func (s *Server) me(w http.ResponseWriter, r *http.Request) error {
	respond(w, 200, who(r))
	return nil
}
func (s *Server) ensureCatalog(ctx context.Context, a account) error {
	if !a.Enabled {
		return nil
	}
	_, err := s.DB.Exec(ctx, `insert into catalog.catalogs(owner_id,name) values($1,$2) on conflict(owner_id) do nothing`, a.ID, "Catálogo de "+a.Name)
	return err
}
func (s *Server) catalogs(w http.ResponseWriter, r *http.Request) error {
	a := who(r)
	if err := s.ensureCatalog(r.Context(), a); err != nil {
		return err
	}
	rows, err := s.DB.Query(r.Context(), `select c.id::text,c.name,c.owner_id=$1 from catalog.catalogs c join catalog.users u on u.id=c.owner_id left join catalog.access_grants g on g.email=u.email where (c.owner_id=$1 or exists(select 1 from catalog.members m where m.catalog_id=c.id and m.email=$2)) and coalesce(g.enabled,$3) order by c.created_at`, a.ID, a.Email, s.Config.AutoActivate)
	if err != nil {
		return err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, name string
		var owner bool
		if err = rows.Scan(&id, &name, &owner); err != nil {
			return err
		}
		out = append(out, map[string]any{"id": id, "name": name, "owner": owner})
	}
	if err = rows.Err(); err != nil {
		return err
	}
	respond(w, 200, out)
	return nil
}
func (s *Server) access(r *http.Request, ownerOnly bool) error {
	a := who(r)
	if _, err := uuid.Parse(r.PathValue("catalog")); err != nil {
		return problem(400, "Identificador de catálogo inválido.")
	}
	if b := r.PathValue("book"); b != "" {
		if _, err := uuid.Parse(b); err != nil {
			return problem(400, "Identificador de livro inválido.")
		}
	}
	var owner, enabled bool
	err := s.DB.QueryRow(r.Context(), `select c.owner_id=$2,coalesce(g.enabled,$4) from catalog.catalogs c join catalog.users u on u.id=c.owner_id left join catalog.access_grants g on g.email=u.email where c.id=$1 and (c.owner_id=$2 or exists(select 1 from catalog.members m where m.catalog_id=c.id and m.email=$3))`, r.PathValue("catalog"), a.ID, a.Email, s.Config.AutoActivate).Scan(&owner, &enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return problem(404, "Catálogo não encontrado.")
	}
	if err != nil {
		return err
	}
	if !enabled {
		return problem(403, "O acesso cloud deste catálogo não está ativo.")
	}
	if ownerOnly && !owner {
		return problem(403, "Somente o responsável pode gerenciar o compartilhamento.")
	}
	return nil
}
func normalizedEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	a, err := mail.ParseAddress(value)
	if err != nil || a.Address != value || len(value) > 254 {
		return "", problem(400, "Informe um e-mail válido.")
	}
	return value, nil
}

func fmtErrorType(err error) string {
	var pg interface{ SQLState() string }
	if errors.As(err, &pg) {
		return "database:" + pg.SQLState()
	}
	return "internal"
}
