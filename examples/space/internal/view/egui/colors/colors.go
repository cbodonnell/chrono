package colors

import (
	"image/color"
	"math"
)

// Layout constants
const (
	ContentWidth  = 80
	ContentHeight = 30
)

// Color palette
var (
	Primary   = color.RGBA{0x5f, 0x5f, 0xff, 0xff} // Blue
	Success   = color.RGBA{0x00, 0xd7, 0x87, 0xff} // Green
	Warning   = color.RGBA{0xff, 0xaf, 0x00, 0xff} // Orange
	Danger    = color.RGBA{0xff, 0x00, 0x00, 0xff} // Red
	Muted     = color.RGBA{0x58, 0x58, 0x58, 0xff} // Gray
	White     = color.RGBA{0xff, 0xff, 0xff, 0xff} // White
	Black     = color.RGBA{0x00, 0x00, 0x00, 0xff} // Black

	// Space-specific colors
	Yellow = color.RGBA{0xff, 0xd7, 0x00, 0xff} // Star color
	Cyan   = color.RGBA{0x00, 0xff, 0xff, 0xff} // Event highlight

	// Mass gradient colors (low to high mass)
	MassVeryLow  = color.RGBA{0x44, 0x44, 0x66, 0xff} // Dim blue-gray (debris)
	MassLow      = color.RGBA{0x00, 0x99, 0xcc, 0xff} // Cyan (small asteroids)
	MassMedium   = color.RGBA{0x00, 0xcc, 0x66, 0xff} // Green (large asteroids)
	MassHigh     = color.RGBA{0xcc, 0xcc, 0x00, 0xff} // Yellow (small planets)
	MassVeryHigh = color.RGBA{0xff, 0x99, 0x00, 0xff} // Orange (large planets)
	MassExtreme  = color.RGBA{0xff, 0x44, 0x00, 0xff} // Red-orange (stars)

	// Background
	Background = color.RGBA{0x00, 0x00, 0x00, 0xff}
)

// MassColor returns a color based on mass using a logarithmic scale.
// Mass ranges: debris (0.1-0.5), asteroids (0.5-3), planets (5-50), stars (500-2000+)
func MassColor(mass float64) color.RGBA {
	if mass <= 0 {
		return MassVeryLow
	}

	// Use log scale: log10(0.1) = -1, log10(1) = 0, log10(10) = 1, log10(1000) = 3
	logMass := math.Log10(mass)

	// Map log mass to 0-1 range: -1 (0.1) to 3.5 (3000+)
	t := (logMass + 1) / 4.5
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}

	// Interpolate through gradient stops
	return interpolateGradient(t)
}

func interpolateGradient(t float64) color.RGBA {
	// Gradient stops at t=0, 0.2, 0.4, 0.6, 0.8, 1.0
	stops := []color.RGBA{
		MassVeryLow,  // t=0.0 (mass ~0.1)
		MassLow,      // t=0.2 (mass ~1)
		MassMedium,   // t=0.4 (mass ~10)
		MassHigh,     // t=0.6 (mass ~100)
		MassVeryHigh, // t=0.8 (mass ~1000)
		MassExtreme,  // t=1.0 (mass ~3000+)
	}

	// Find which segment we're in
	segment := t * float64(len(stops)-1)
	idx := int(segment)
	if idx >= len(stops)-1 {
		return stops[len(stops)-1]
	}

	// Interpolate within segment
	frac := segment - float64(idx)
	c1, c2 := stops[idx], stops[idx+1]

	return color.RGBA{
		R: uint8(float64(c1.R) + frac*(float64(c2.R)-float64(c1.R))),
		G: uint8(float64(c1.G) + frac*(float64(c2.G)-float64(c1.G))),
		B: uint8(float64(c1.B) + frac*(float64(c2.B)-float64(c1.B))),
		A: 0xff,
	}
}
