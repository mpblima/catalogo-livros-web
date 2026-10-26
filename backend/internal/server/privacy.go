package server

import "net/http"

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
