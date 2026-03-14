package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cbodonnell/chrono/internal/server"
	"github.com/cbodonnell/chrono/pkg/config"
	"github.com/cbodonnell/chrono/pkg/store"
)

func main() {
	// Create a context that cancels on SIGINT or SIGTERM
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var (
		configFile = flag.String("config", "", "Path to config file (required)")
	)
	flag.Parse()

	if *configFile == "" {
		log.Fatal("config file is required: use -config flag")
	}

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Create entity store
	es, err := store.NewEmbeddedStore(cfg)
	if err != nil {
		log.Fatalf("failed to create entity store : %v", err)
	}
	defer es.Close(ctx)

	// Create and start server
	srv := server.New(es, server.Config{
		Addr: cfg.Server.Addr,
	})

	go func() {
		log.Printf("chrono server listening on %s", cfg.Server.Addr)
		if err := srv.Start(); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	log.Println("shutdown complete")
}
