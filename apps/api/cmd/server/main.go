package main

import (
	"context"
	"log"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/auth"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/db"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/server"
)

func main() {
	cfg := config.MustLoad()

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer pool.Close()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		log.Fatalf("acquire connection failed: %v", err)
	}
	if err := db.MigrateUp(ctx, conn.Conn(), db.MigrationDir()); err != nil {
		conn.Release()
		log.Fatalf("migrations failed: %v", err)
	}
	conn.Release()

	auth.InitJWT(cfg.JWTSecret)

	srv := server.NewWithDB(cfg, pool)

	log.Printf("starting server on port %s", cfg.Port)
	if err := srv.Run(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
