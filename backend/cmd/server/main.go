package main

import (
	"context"
	"embed"
	"log/slog"
	"net/http"
	"os"
	"time"

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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	repo := repository.New(dbpool)

	// Migrate any existing disk uploads to PostgreSQL repository
	if migratedCount, err := repo.MigrateDiskUploadsToDB(context.Background()); err != nil {
		logger.Error("failed to migrate disk uploads to database", "error", err)
	} else if migratedCount > 0 {
		logger.Info("migrated disk upload files to PostgreSQL database repository", "count", migratedCount)
	}

	svc := service.New(repo, cfg.GeminiAPIKey, cfg.VAPIDPublicKey, cfg.VAPIDPrivateKey, cfg.VAPIDContact)

	// Execute comprehensive data & upload asset cleanup on server startup
	if stats, err := svc.RunDataCleanup(context.Background()); err != nil {
		logger.Error("failed to run initial data cleanup", "error", err)
	} else {
		logger.Info("completed initial data and asset cleanup", "stats", stats)
	}

	// Periodic background maintenance ticker (every 6 hours)
	go func() {
		ctx := context.Background()
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if stats, err := svc.RunDataCleanup(ctx); err != nil {
				logger.Error("background data cleanup maintenance error", "error", err)
			} else {
				logger.Info("completed background data cleanup maintenance", "stats", stats)
			}
		}
	}()

	api := handler.New(svc, cfg.JWTSecret, cfg.VAPIDPublicKey)

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
