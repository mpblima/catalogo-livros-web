// Package migrate applies numbered SQL files with checksums, locking and atomic history.
package migrate

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

//go:embed baseline.json
var baseline []byte
var fileName = regexp.MustCompile(`^[0-9]{3}_[a-z0-9_]+\.sql$`)
var start = regexp.MustCompile(`(?is)^\s*begin\s*;`)
var end = regexp.MustCompile(`(?is)commit\s*;\s*$`)

type Options struct{ Directory, BaselineThrough string }
type column struct {
	Table    string `json:"table_name"`
	Position int    `json:"ordinal_position"`
	Name     string `json:"column_name"`
	Type     string `json:"data_type"`
	Nullable string `json:"is_nullable"`
}

func verifyBaseline(ctx context.Context, c *pgx.Conn) error {
	rows, err := c.Query(ctx, `select table_name,ordinal_position,column_name,data_type,is_nullable from information_schema.columns where table_schema='catalog' and table_name in ('users','access_grants','service_settings','catalogs','members','books','comments','sync_links','email_outbox','audit_events','privacy_requests') order by table_name,ordinal_position`)
	if err != nil {
		return err
	}
	defer rows.Close()
	actual := []column{}
	for rows.Next() {
		var col column
		if err = rows.Scan(&col.Table, &col.Position, &col.Name, &col.Type, &col.Nullable); err != nil {
			return err
		}
		actual = append(actual, col)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	var expected []column
	if err = json.Unmarshal(baseline, &expected); err != nil {
		return err
	}
	a, _ := json.Marshal(actual)
	b, _ := json.Marshal(expected)
	if string(a) != string(b) {
		return fmt.Errorf("existing schema differs from the verified 001/002 baseline; automatic adoption refused")
	}
	var safe bool
	err = c.QueryRow(ctx, `select count(*)=11 and bool_and(c.relrowsecurity) from pg_class c join pg_namespace n on n.oid=c.relnamespace where n.nspname='catalog' and c.relkind='r' and c.relname in ('users','access_grants','service_settings','catalogs','members','books','comments','sync_links','email_outbox','audit_events','privacy_requests')`).Scan(&safe)
	if err != nil {
		return err
	}
	if !safe {
		return fmt.Errorf("baseline requires RLS on all original tables")
	}
	return nil
}
func Run(ctx context.Context, c *pgx.Conn, options Options) ([]string, error) {
	entries, err := os.ReadDir(options.Directory)
	if err != nil {
		return nil, err
	}
	names := []string{}
	for _, e := range entries {
		if fileName.MatchString(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil, fmt.Errorf("no migrations found")
	}
	if options.BaselineThrough != "" && options.BaselineThrough != "002_delivery_audit_privacy.sql" {
		return nil, fmt.Errorf("only the verified original 001/002 baseline can be adopted")
	}
	if _, err = c.Exec(ctx, `select pg_advisory_lock(70862026)`); err != nil {
		return nil, err
	}
	defer c.Exec(context.Background(), `select pg_advisory_unlock(70862026)`)
	_, err = c.Exec(ctx, `create schema if not exists catalog_migrations;create table if not exists catalog_migrations.history(name text primary key,checksum text not null,applied_at timestamptz not null default now());alter table catalog_migrations.history enable row level security;revoke all on schema catalog_migrations from public,anon,authenticated;revoke all on catalog_migrations.history from public,anon,authenticated`)
	if err != nil {
		return nil, err
	}
	history := map[string]string{}
	rows, err := c.Query(ctx, `select name,checksum from catalog_migrations.history`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var name, sum string
		if err = rows.Scan(&name, &sum); err != nil {
			rows.Close()
			return nil, err
		}
		history[name] = sum
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return nil, err
	}
	available := map[string]bool{}
	for _, name := range names {
		available[name] = true
	}
	for name := range history {
		if !available[name] {
			return nil, fmt.Errorf("applied migration %s is missing from this checkout", name)
		}
	}
	adopting := options.BaselineThrough != "" && len(history) == 0
	if adopting {
		if err = verifyBaseline(ctx, c); err != nil {
			return nil, err
		}
	}
	applied := []string{}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(options.Directory, name))
		if err != nil {
			return applied, err
		}
		hash := sha256.Sum256(data)
		sum := hex.EncodeToString(hash[:])
		if previous, ok := history[name]; ok {
			if previous != sum {
				return applied, fmt.Errorf("checksum mismatch for applied migration %s", name)
			}
			continue
		}
		tx, err := c.Begin(ctx)
		if err != nil {
			return applied, err
		}
		if !(adopting && name <= options.BaselineThrough) {
			sql := strings.TrimSpace(string(data))
			sql = start.ReplaceAllString(sql, "")
			sql = end.ReplaceAllString(sql, "")
			_, err = tx.Exec(ctx, sql)
		}
		if err == nil {
			_, err = tx.Exec(ctx, `insert into catalog_migrations.history(name,checksum)values($1,$2)`, name, sum)
		}
		if err == nil {
			err = tx.Commit(ctx)
		} else {
			tx.Rollback(ctx)
		}
		if err != nil {
			return applied, fmt.Errorf("migration %s failed: %w", name, err)
		}
		applied = append(applied, name)
	}
	return applied, nil
}
