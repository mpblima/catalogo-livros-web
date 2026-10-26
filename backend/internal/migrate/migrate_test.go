package migrate

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func database(t *testing.T) *pgx.Conn {
	t.Helper()
	if os.Getenv("TEST_DATABASE_URL") == "" {
		t.Skip("disposable TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("migrations_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "create database "+name); err != nil {
		t.Fatal(err)
	}
	cfg := admin.Config().Copy()
	cfg.Database = name
	c, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close(ctx); admin.Exec(ctx, "drop database "+name); admin.Close(ctx) })
	return c
}
func files(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	entries, err := filepath.Glob("../../../supabase/migrations/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range entries {
		data, _ := os.ReadFile(p)
		if err = os.WriteFile(filepath.Join(dir, filepath.Base(p)), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
func TestMigrationsAtomicAndChecksummed(t *testing.T) {
	c := database(t)
	ctx := context.Background()
	dir := files(t)
	names, _ := filepath.Glob(filepath.Join(dir, "*.sql"))
	applied, err := Run(ctx, c, Options{Directory: dir})
	if err != nil || len(applied) != len(names) {
		t.Fatalf("initial migrations %v %v", applied, err)
	}
	again, err := Run(ctx, c, Options{Directory: dir})
	if err != nil || len(again) != 0 {
		t.Fatalf("rerun %v %v", again, err)
	}
	if err = os.WriteFile(filepath.Join(dir, "999_failed.sql"), []byte("begin;create table catalog.must_rollback(id int);select invalid_statement;commit;"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = Run(ctx, c, Options{Directory: dir}); err == nil {
		t.Fatal("bad SQL accepted")
	}
	var exists bool
	c.QueryRow(ctx, `select to_regclass('catalog.must_rollback') is not null`).Scan(&exists)
	if exists {
		t.Fatal("partial DDL survived failed migration")
	}
	os.Remove(filepath.Join(dir, "999_failed.sql"))
	p := filepath.Join(dir, "001_initial.sql")
	data, _ := os.ReadFile(p)
	os.WriteFile(p, append(data, []byte("\n-- changed")...), 0600)
	if _, err = Run(ctx, c, Options{Directory: dir}); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("changed migration accepted: %v", err)
	}
}
func TestAdoptVerifiedBaseline(t *testing.T) {
	c := database(t)
	ctx := context.Background()
	dir := files(t)
	for _, name := range []string{"001_initial.sql", "002_delivery_audit_privacy.sql"} {
		data, _ := os.ReadFile(filepath.Join(dir, name))
		if _, err := c.Exec(ctx, string(data)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := Run(ctx, c, Options{Directory: dir, BaselineThrough: "002_delivery_audit_privacy.sql"}); err != nil {
		t.Fatal(err)
	}
}
func TestRejectDriftedBaseline(t *testing.T) {
	c := database(t)
	ctx := context.Background()
	dir := files(t)
	for _, name := range []string{"001_initial.sql", "002_delivery_audit_privacy.sql"} {
		data, _ := os.ReadFile(filepath.Join(dir, name))
		c.Exec(ctx, string(data))
	}
	c.Exec(ctx, `alter table catalog.books drop column summary`)
	if _, err := Run(ctx, c, Options{Directory: dir, BaselineThrough: "002_delivery_audit_privacy.sql"}); err == nil {
		t.Fatal("schema drift was adopted")
	}
}
