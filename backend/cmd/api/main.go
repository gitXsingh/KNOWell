package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/gitXsingh/knowell/backend/internal/common/config"
	dbconn "github.com/gitXsingh/knowell/backend/internal/common/db"
	"github.com/gitXsingh/knowell/backend/internal/common/migrations"
	appserver "github.com/gitXsingh/knowell/backend/internal/common/server"
)

func main() {
	cfg := config.Load()
	database, err := dbconn.Open(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	if err := migrations.Run(database, cfg.MigrationsDir); err != nil {
		log.Fatal(err)
	}

	server := appserver.New(cfg, database)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.Start(ctx); err != nil {
		log.Fatal(err)
	}

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatal(err)
	}
}
