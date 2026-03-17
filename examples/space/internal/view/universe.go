package view

import (
	"sort"

	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/colors"
	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/components"
	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/renderer"
	"github.com/cbodonnell/chrono/pkg/entity"
)

// Universe coordinate space (0-100)
const universeSize = 100.0

// Mass kind constants (derived from mass + composition)
const (
	kindStar     = "star"     // Gas, mass >= 500 (fusion)
	kindGiant    = "giant"    // Gas, mass 50-500 (gas giant)
	kindCloud    = "cloud"    // Gas, mass < 50 (gas remnant)
	kindPlanet   = "planet"   // Rock, mass >= 15 (spherical)
	kindAsteroid = "asteroid" // Rock, mass 1.5-15 (irregular)
	kindDebris   = "debris"   // Rock, mass < 1.5 (fragments)
)

// UniverseRenderer renders bodies to a pixel canvas
type UniverseRenderer struct {
	canvas *components.GraphicsCanvas
}

// NewUniverseRenderer creates a new universe renderer at the given cell position
func NewUniverseRenderer(x, y, width, height int) *UniverseRenderer {
	return &UniverseRenderer{
		canvas: components.NewGraphicsCanvas(x, y, width, height),
	}
}

// kindPriority returns render priority (lower = drawn first, higher = drawn on top)
func kindPriority(kind string) int {
	switch kind {
	case kindDebris:
		return 0
	case kindCloud:
		return 1
	case kindAsteroid:
		return 2
	case kindPlanet:
		return 3
	case kindGiant:
		return 4
	case kindStar:
		return 5
	default:
		return 0
	}
}

// Draw renders all masses to the canvas
func (u *UniverseRenderer) Draw(masses []*entity.Entity) {
	// Clear canvas
	u.canvas.Clear(colors.Background)

	// Sort by kind priority (debris first, stars last = stars on top)
	sorted := make([]*entity.Entity, len(masses))
	copy(sorted, masses)
	sort.Slice(sorted, func(i, j int) bool {
		return kindPriority(sorted[i].Fields["kind"].S) < kindPriority(sorted[j].Fields["kind"].S)
	})

	// Get canvas pixel dimensions for coordinate mapping
	pixelW, pixelH := u.canvas.PixelSize()

	// Draw each mass
	for _, mass := range sorted {
		// Skip dead masses
		if !mass.Fields["alive"].B {
			continue
		}

		// Get position and properties
		x := mass.Fields["x"].F
		y := mass.Fields["y"].F
		kind := mass.Fields["kind"].S
		massVal := mass.Fields["mass"].F

		// Convert universe coordinates to pixel coordinates
		px := x / universeSize * float64(pixelW)
		py := y / universeSize * float64(pixelH)

		// Clamp to canvas bounds
		if px < 0 {
			px = 0
		}
		if px >= float64(pixelW) {
			px = float64(pixelW) - 1
		}
		if py < 0 {
			py = 0
		}
		if py >= float64(pixelH) {
			py = float64(pixelH) - 1
		}

		// Get radius and color based on kind and mass
		radius := colors.BodyRadius(massVal)
		clr := colors.BodyColor(kind, massVal)

		// Draw star glow effect
		if kind == kindStar {
			// Draw outer glow first
			u.canvas.DrawGlow(px, py, radius, radius*2.5, colors.StarGlow)
		}

		// Draw the body
		u.canvas.DrawCircle(px, py, radius, clr)
	}
}

// Register registers the canvas for compositing with the terminal
func (u *UniverseRenderer) Register(term *renderer.Terminal) {
	px, py := u.canvas.PixelPosition()
	term.RegisterComposite(u.canvas.Image(), px, py)
}

// DrawBox draws the box border around the universe (using terminal characters)
func (u *UniverseRenderer) DrawBox(term *renderer.Terminal) {
	// Draw box around the universe canvas
	term.DrawBoxWithTitle(u.canvas.X-1, u.canvas.Y-1, u.canvas.Width+2, u.canvas.Height+2, "Universe", colors.Muted)
}

// Position returns the cell position of the canvas
func (u *UniverseRenderer) Position() (x, y int) {
	return u.canvas.X, u.canvas.Y
}

// Size returns the cell size of the canvas
func (u *UniverseRenderer) Size() (width, height int) {
	return u.canvas.Width, u.canvas.Height
}
