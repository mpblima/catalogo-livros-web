package server

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mpblima/catalogo-livros-web/backend/internal/migrate"
)

// Runs the real Java client against the Go HTTP API; identities exist only in this test.
func TestDesktopClient(t *testing.T) {
	desktop := os.Getenv("DESKTOP_REPO")
	if desktop == "" || os.Getenv("TEST_DATABASE_URL") == "" {
		t.Skip("DESKTOP_REPO and disposable TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("java_sync_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "create database "+name); err != nil {
		t.Fatal(err)
	}
	cfg := admin.Config().Copy()
	cfg.Database = name
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = migrate.Run(ctx, conn, migrate.Options{Directory: "../../../supabase/migrations"}); err != nil {
		t.Fatal(err)
	}
	conn.Close(ctx)
	pc, _ := pgxpool.ParseConfig(os.Getenv("TEST_DATABASE_URL"))
	pc.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { pool.Close(); admin.Exec(ctx, "drop database "+name); admin.Close(ctx) }()
	identity := func(ctx context.Context, id string) (Identity, error) {
		return Identity{id, id + "@example.com", id}, nil
	}
	s := Server{DB: pool, Auth: identity, LookupIdentity: identity, Config: Config{BootstrapAdminEmail: "owner@example.com", AutoActivate: true, FrontendURL: "http://localhost:5173"}}
	httpServer := httptest.NewServer(s.Handler())
	defer httpServer.Close()
	command := exec.Command("mvn", "-q", "-Dtest=CloudSyncTest", "-Dcatalogo.test.cloud.api="+httpServer.URL, "test")
	command.Dir = desktop
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("Java/Go integration failed: %v\n%s", err, out)
	}
}
