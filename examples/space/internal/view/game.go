package view

import (
	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/input"
	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/renderer"
	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/screen"
	"github.com/cbodonnell/chrono/pkg/store"
	"github.com/hajimehoshi/ebiten/v2"
)

// Game represents the main game state
type Game struct {
	terminal *renderer.Terminal
	input    *input.Handler
	screen   *Screen
}

// New creates a new game instance
func New(es *store.EntityStore) (*Game, error) {
	terminal, err := renderer.NewTerminal()
	if err != nil {
		return nil, err
	}

	inputHandler := input.NewHandler()
	viewScreen := NewScreen(es)

	return &Game{
		terminal: terminal,
		input:    inputHandler,
		screen:   viewScreen,
	}, nil
}

// Update implements ebiten.Game
func (g *Game) Update() error {
	state := g.screen.Update(g.input)
	if state == screen.StateQuit {
		return ebiten.Termination
	}
	return nil
}

// Draw implements ebiten.Game
func (g *Game) Draw(ebitenScreen *ebiten.Image) {
	g.terminal.Clear()
	g.screen.Draw(g.terminal)
	g.terminal.Draw(ebitenScreen)
}

// Layout implements ebiten.Game
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return renderer.ScreenWidth(), renderer.ScreenHeight()
}
