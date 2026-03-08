package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cbodonnell/chrono/internal/config"
	"github.com/cbodonnell/chrono/internal/server"
	"github.com/cbodonnell/chrono/pkg/store"
	badgerstore "github.com/cbodonnell/chrono/pkg/store/badger"
	"github.com/dgraph-io/badger/v4"
)

func main() {
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

	// Build index registry from config
	registry, err := cfg.BuildRegistry()
	if err != nil {
		log.Fatalf("failed to build registry: %v", err)
	}

	// Open BadgerDB
	opts := badger.DefaultOptions(cfg.Storage.DataDir)
	opts.Logger = nil // Disable badger's verbose logging
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create storage backends
	kv := badgerstore.NewKVStore(db)
	idx := badgerstore.NewIndexStore(db)

	// Create entity store
	es := store.NewEntityStore(kv, idx, registry, store.NewMsgpackSerializer())
	defer es.Close()

	// TODO: should this happen in NewEntityStore (maybe conditionally)
	// Sync indexes (checks for config changes and reindexes if needed)
	if err := es.SyncIndexes(); err != nil {
		log.Fatalf("failed to sync indexes: %v", err)
	}

	// Create and start server
	srv := server.New(es, server.Config{
		Addr:     cfg.Server.Addr,
		Registry: registry,
	})

	// Handle graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("chrono server listening on %s", cfg.Server.Addr)
		if err := srv.Start(); err != nil {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-done
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}

	log.Println("shutdown complete")
}
