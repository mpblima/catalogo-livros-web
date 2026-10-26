package main

import (
	"context"
	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mpblima/catalogo-livros-web/backend/internal/server"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	for _, key := range []string{"DATABASE_URL", "CLERK_SECRET_KEY", "BOOTSTRAP_ADMIN_EMAIL", "ALLOWED_ORIGINS", "FRONTEND_URL"} {
		if os.Getenv(key) == "" {
			slog.Error("Missing environment variable", "name", key)
			os.Exit(1)
		}
	}
	origins := strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
		if origins[i] == "" || origins[i] == "*" {
			slog.Error("Explicit origins required")
			os.Exit(1)
		}
	}
	clerk.SetKey(os.Getenv("CLERK_SECRET_KEY"))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	config, err := pgxpool.ParseConfig(os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("Invalid database configuration")
		os.Exit(1)
	}
	if config.ConnConfig.Host != "localhost" && config.ConnConfig.Host != "127.0.0.1" {
		if config.ConnConfig.TLSConfig == nil || config.ConnConfig.TLSConfig.InsecureSkipVerify {
			slog.Error("Database TLS certificate verification is required; use sslmode=verify-full")
			os.Exit(1)
		}
	}
	config.MaxConns = 5
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		slog.Error("Database initialization failed")
		os.Exit(1)
	}
	defer pool.Close()
	app := server.Server{DB: pool, Auth: server.ClerkAuth(origins), Config: server.Config{BootstrapAdminEmail: strings.ToLower(strings.TrimSpace(os.Getenv("BOOTSTRAP_ADMIN_EMAIL"))), Origins: origins, AutoActivate: os.Getenv("AUTO_ACTIVATE_ACCOUNTS") == "true", FrontendURL: os.Getenv("FRONTEND_URL")}}
	if os.Getenv("SENDER_API_KEY") != "" && os.Getenv("SENDER_FROM_EMAIL") != "" {
		app.Mailer = server.SenderMailer{APIKey: os.Getenv("SENDER_API_KEY"), FromEmail: os.Getenv("SENDER_FROM_EMAIL"), FromName: "Catálogo de Livros"}
		go func() {
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					for i := 0; i < 10; i++ {
						jobCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
						worked, err := app.DeliverNext(jobCtx)
						cancel()
						if err != nil {
							slog.Error("Email queue processing failed")
							break
						}
						if !worked {
							break
						}
					}
				}
			}
		}()
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := http.Server{Addr: ":" + port, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 20 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		slog.Info("API listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
			stop()
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}
