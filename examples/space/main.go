// Package main demonstrates chrono's time-series capabilities through a procedural
// universe simulation with a terminal UI for scrubbing through time.
//
// Usage:
//
//	go run . generate  # Generate universe data
//	go run . view      # View universe with time scrubbing
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cbodonnell/chrono/examples/space/internal/generate"
	"github.com/cbodonnell/chrono/examples/space/internal/view"
	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/renderer"
	"github.com/cbodonnell/chrono/pkg/config"
	"github.com/cbodonnell/chrono/pkg/store"
	"github.com/hajimehoshi/ebiten/v2"
)

const configPath = "config.yaml"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "generate":
		runGenerate()
	case "view":
		runView()
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: space <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  generate  Generate a procedural universe")
	fmt.Println("  view      View the universe with time scrubbing")
}

func runGenerate() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Remove old data
	if err := os.RemoveAll("./data"); err != nil {
		log.Fatalf("failed to remove old data: %v", err)
	}

	// Load config
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Create store
	es, err := store.NewEmbeddedStore(cfg)
	if err != nil {
		log.Fatalf("failed to create store: %v", err)
	}
	defer es.Close(ctx)

	// Generate universe
	seed := time.Now().UnixNano()
	gen := generate.New(es, seed)
	if err := gen.Generate(); err != nil {
		log.Fatalf("generation failed: %v", err)
	}

	fmt.Println("Universe generated successfully!")
	fmt.Println("Run 'go run . view' to explore it.")
}

func runView() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load config
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Create store
	es, err := store.NewEmbeddedStore(cfg)
	if err != nil {
		log.Fatalf("failed to create store: %v", err)
	}
	defer es.Close(ctx)

	// Create game
	game, err := view.New(es)
	if err != nil {
		log.Fatalf("failed to create game: %v", err)
	}

	// Configure window
	ebiten.SetWindowSize(renderer.WindowWidth(), renderer.WindowHeight())
	ebiten.SetWindowTitle("Space - Chrono Example")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	// Run game
	if err := ebiten.RunGame(game); err != nil {
		log.Fatalf("game error: %v", err)
	}
}
