package view

import (
	"image/color"
	"sort"

	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/colors"
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
	kindPlanet   = "planet"   // Rock, mass >= 50 (spherical)
	kindAsteroid = "asteroid" // Rock, mass 3-50 (irregular)
	kindDebris   = "debris"   // Rock, mass < 3 (fragments)
)

// Grid renders the universe visualization
type Grid struct {
	X, Y          int
	Width, Height int
}

// NewGrid creates a new universe grid
func NewGrid(x, y, width, height int) *Grid {
	return &Grid{
		X:      x,
		Y:      y,
		Width:  width,
		Height: height,
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

// Draw renders the universe grid with all masses
func (g *Grid) Draw(term *renderer.Terminal, masses []*entity.Entity) {
	// Draw box around the universe
	term.DrawBoxWithTitle(g.X-1, g.Y-1, g.Width+2, g.Height+2, "Universe", colors.Muted)

	// Sort by kind priority (stars first, debris last) - higher priority drawn first
	sorted := make([]*entity.Entity, len(masses))
	copy(sorted, masses)
	sort.Slice(sorted, func(i, j int) bool {
		return kindPriority(sorted[i].Fields["kind"].S) > kindPriority(sorted[j].Fields["kind"].S)
	})

	// Track occupied cells to avoid overdraw
	occupied := make(map[int]bool)

	// Draw each mass
	for _, mass := range sorted {
		// Skip dead masses
		if !mass.Fields["alive"].B {
			continue
		}

		// Get position
		x := mass.Fields["x"].F
		y := mass.Fields["y"].F
		kind := mass.Fields["kind"].S

		// Convert to grid coordinates
		gridX := g.X + int(x/universeSize*float64(g.Width))
		gridY := g.Y + int(y/universeSize*float64(g.Height))

		// Clamp to grid bounds
		if gridX < g.X {
			gridX = g.X
		}
		if gridX >= g.X+g.Width {
			gridX = g.X + g.Width - 1
		}
		if gridY < g.Y {
			gridY = g.Y
		}
		if gridY >= g.Y+g.Height {
			gridY = g.Y + g.Height - 1
		}

		// Skip if cell already occupied by higher priority entity
		cellKey := gridY*1000 + gridX
		if occupied[cellKey] {
			continue
		}
		occupied[cellKey] = true

		// Get mass value for coloring
		massVal := mass.Fields["mass"].F

		// Get character and color based on kind and mass
		ch, clr := g.getMassAppearance(kind, massVal)

		// Draw the mass
		term.SetCharWithColor(gridX, gridY, ch, clr)
	}
}

func (g *Grid) getMassAppearance(kind string, mass float64) (rune, color.RGBA) {
	// Color based on mass (log scale gradient)
	clr := colors.MassColor(mass)

	// Character based on kind
	// Gas bodies: radiating/star-like shapes
	// Rock bodies: solid geometric shapes
	switch kind {
	case kindStar:
		return '\u2605', clr // ★ (filled star - fusion!)
	case kindGiant:
		return '\u2726', clr // ✦ (4-pointed star - gas giant)
	case kindCloud:
		return '*', clr // * (asterisk - gas cloud)
	case kindPlanet:
		return '\u25C9', clr // ◉ (fisheye - rocky planet)
	case kindAsteroid:
		return '\u2022', clr // • (bullet - asteroid, smaller than ●)
	case kindDebris:
		return '\u00B7', clr // · (middle dot - debris)
	default:
		return '?', clr
	}
}
