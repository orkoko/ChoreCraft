package main

import (
	"context"
	"embed"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"

	"ChoreCraft/internal/config"
	"ChoreCraft/internal/handler"
	"ChoreCraft/internal/repository"
	"ChoreCraft/internal/service"
)

//go:embed schema.sql
var schemaSQL embed.FS

// @title ChoreCraft API
// @version 1.0
// @description This is the backend server for the ChoreCraft application.
// @host localhost:8080
// @BasePath /api
//
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-User-ID
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	dbpool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		logger.Error("unable to connect to database", "error", err)
		os.Exit(1)
	}
	defer dbpool.Close()

	if err := executeSchema(dbpool, logger); err != nil {
		logger.Error("failed to execute schema", "error", err)
		os.Exit(1)
	}

	r := setupRouter(dbpool, cfg)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	logger.Info("starting server", "address", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("could not start server", "error", err)
		os.Exit(1)
	}
}

func setupRouter(dbpool *pgxpool.Pool, cfg *config.Config) *chi.Mux {
	repo := repository.New(dbpool)
	svc := service.New(repo, cfg.GeminiAPIKey)
	api := handler.New(svc)

	r := chi.NewRouter()
	api.RegisterRoutes(r)
	return r
}

func executeSchema(dbpool *pgxpool.Pool, logger *slog.Logger) error {
	logger.Info("executing database schema")
	sql, err := schemaSQL.ReadFile("schema.sql")
	if err != nil {
		return err
	}
	_, err = dbpool.Exec(context.Background(), string(sql))
	if err != nil {
		return err
	}
	logger.Info("database schema executed successfully")
	return nil
}
