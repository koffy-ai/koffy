package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"koffy/internal/aigateway"
	"koffy/internal/config"
	"koffy/internal/storage"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate("koffy-gateway"); err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := storage.OpenMySQL(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	server := aigateway.NewServer(cfg, db)

	log.Printf("koffy-gateway listening on %s", cfg.AIGatewayAddr)
	if err := http.ListenAndServe(cfg.AIGatewayAddr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}
