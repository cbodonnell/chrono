package screen

import (
	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/input"
	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/renderer"
)

// State represents the current screen/view
type State int

const (
	StateView State = iota
	StateQuit
)

// Screen is the interface that all screens must implement
type Screen interface {
	// Update handles input and updates screen state
	Update(input *input.Handler) State

	// Draw renders the screen to the terminal
	Draw(term *renderer.Terminal)
}
