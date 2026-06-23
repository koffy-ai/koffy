package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"koffy/internal/billing"
	"koffy/internal/config"
	"koffy/internal/storage"
)

func main() {
	cfg := config.Load()
	if err := cfg.Validate("koffy-billing-api"); err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	db, err := storage.OpenMySQL(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	server := billing.NewServer(cfg, db)
	server.StartBackgroundJobs(context.Background())

	log.Printf("koffy-billing-api listening on %s", cfg.BillingAPIAddr)
	if err := http.ListenAndServe(cfg.BillingAPIAddr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}
