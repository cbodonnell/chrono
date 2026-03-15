package input

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Key represents an input key or action
type Key int

const (
	KeyNone Key = iota
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyEnter
	KeyEscape
	KeyQuit // Ctrl+C or Q
	KeyS
	KeySpace
)

// Handler manages keyboard input
type Handler struct{}

// NewHandler creates a new input handler
func NewHandler() *Handler {
	return &Handler{}
}

// GetPressedKey returns the currently pressed navigation key
func (h *Handler) GetPressedKey() Key {
	// Check for quit (Ctrl+C or Q)
	if ebiten.IsKeyPressed(ebiten.KeyControl) && inpututil.IsKeyJustPressed(ebiten.KeyC) {
		return KeyQuit
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		return KeyQuit
	}

	// Navigation keys
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		return KeyUp
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		return KeyDown
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
		return KeyLeft
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyRight) {
		return KeyRight
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		return KeyEnter
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return KeyEscape
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyS) {
		return KeyS
	}
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		return KeySpace
	}

	return KeyNone
}

// KeyRepeat returns true if a key should repeat (for held keys)
func (h *Handler) KeyRepeat(key ebiten.Key, initialDelay, repeatDelay int) bool {
	duration := inpututil.KeyPressDuration(key)
	if duration == 0 {
		return false
	}
	if duration == 1 {
		return true
	}
	if duration >= initialDelay {
		return (duration-initialDelay)%repeatDelay == 0
	}
	return false
}
