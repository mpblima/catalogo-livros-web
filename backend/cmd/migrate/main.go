package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/mpblima/catalogo-livros-web/backend/internal/migrate"
	"os"
	"time"
)

func main() {
	directory := flag.String("dir", "../supabase/migrations", "SQL directory")
	baseline := flag.String("baseline-through", "", "adopt an already installed and verified baseline")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cfg, err := pgx.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil || os.Getenv("DATABASE_URL") == "" {
		fmt.Fprintln(os.Stderr, "Invalid DATABASE_URL configuration")
		os.Exit(1)
	}
	if cfg.Host != "localhost" && cfg.Host != "127.0.0.1" && (cfg.TLSConfig == nil || cfg.TLSConfig.InsecureSkipVerify) {
		fmt.Fprintln(os.Stderr, "Verified database TLS is required")
		os.Exit(1)
	}
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Database connection failed; connection details omitted")
		os.Exit(1)
	}
	defer conn.Close(ctx)
	applied, err := migrate.Run(ctx, conn, migrate.Options{Directory: *directory, BaselineThrough: *baseline})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, name := range applied {
		fmt.Println("Applied:", name)
	}
	fmt.Println("Schema is up to date")
}
