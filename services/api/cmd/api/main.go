package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kinonoja/gobot/services/api/internal/lessons"
	"github.com/kinonoja/gobot/services/api/internal/runnerclient"
	"github.com/kinonoja/gobot/services/api/internal/server"
	"github.com/kinonoja/gobot/services/api/internal/store"
)

func main() {
	port := envOr("PORT", "8081")
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	runnerURL := envOr("RUNNER_URL", "http://localhost:8080")
	contentDir := envOr("CONTENT_DIR", "../../content/courses")
	devAuth, err := strconv.ParseBool(envOr("DEV_AUTH", "false"))
	if err != nil {
		log.Fatalf("DEV_AUTH must be true or false: %v", err)
	}
	runnerTimeout, err := time.ParseDuration(envOr("RUNNER_TIMEOUT", "45s"))
	if err != nil || runnerTimeout <= 0 {
		log.Fatal("RUNNER_TIMEOUT must be a positive duration")
	}
	courses, err := lessons.Load(contentDir)
	if err != nil {
		log.Fatalf("load lessons: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("configure database: %v", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	api := server.NewHandler(courses, store.NewPostgres(pool), runnerclient.New(runnerURL, runnerTimeout), devAuth)
	httpServer := &http.Server{Addr: ":" + port, Handler: api, ReadHeaderTimeout: 5 * time.Second}
	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- httpServer.ListenAndServe()
	}()
	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("serve API: %v", err)
		}
	case <-shutdownCtx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
