package server

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

// Preserve the editable desktop notes field and an attributable shared history.
// Author fields always come from the verified identity, never from client input.
func prepareBookNotes(ctx context.Context, tx pgx.Tx, catalog, previous string, b *Book, a account) (string, error) {
	old := map[string]any{}
	if previous != "" {
		err := tx.QueryRow(ctx, `select metadata from catalog.books where catalog_id=$1 and id=$2`, catalog, previous).Scan(&old)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return "", err
		}
	}
	note := textValue(b.Metadata["notes"])
	changed := note != textValue(old["notes"])
	for _, key := range []string{"notes_author_id", "notes_author_name", "notes_author_email"} {
		delete(b.Metadata, key)
		if value, ok := old[key]; ok {
			b.Metadata[key] = value
		}
	}
	if !changed {
		return "", nil
	}
	b.Metadata["notes_author_id"] = a.ID
	b.Metadata["notes_author_name"] = a.Name
	b.Metadata["notes_author_email"] = a.Email
	return note, nil
}
func recordBookNotes(ctx context.Context, tx pgx.Tx, catalog, book, user, note, origin string) error {
	if note == "" {
		return nil
	}
	_, err := tx.Exec(ctx, `insert into catalog.comments(catalog_id,book_id,author_id,body,origin)values($1,$2,$3,$4,$5)`, catalog, book, user, note, origin)
	return err
}
