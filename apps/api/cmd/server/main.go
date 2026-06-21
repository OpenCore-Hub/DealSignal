package main

import (
	"log"

	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/config"
	"github.com/OpenCore-Hub/DealSignal/apps/api/internal/server"
)

func main() {
	cfg := config.MustLoad()
	srv := server.New(cfg)

	log.Printf("starting server on port %s", cfg.Port)
	if err := srv.Run(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
