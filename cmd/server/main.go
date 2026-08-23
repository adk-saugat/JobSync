package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/saugatadhikari/jobSync/internal/config"
	"github.com/saugatadhikari/jobSync/internal/cloud/server"
)

func main() {
	config.LoadDotEnv()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	ctx := context.Background()
	srv, err := server.NewFromEnv(ctx)
	if err != nil {
		log.Fatalf("server init: %v", err)
	}
	defer func() { _ = srv.DB.Close() }()

	addr := ":" + port
	log.Printf("jobsync server listening on %s", addr)
	if err := http.ListenAndServe(addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
