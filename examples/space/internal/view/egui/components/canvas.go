package components

import (
	"image/color"
	"math"

	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/renderer"
	"github.com/hajimehoshi/ebiten/v2"
)

// GraphicsCanvas provides a pixel-based drawing surface within the terminal grid
type GraphicsCanvas struct {
	X, Y          int           // Position in character cells
	Width, Height int           // Size in character cells
	image         *ebiten.Image // Pixel buffer (Width*CellWidth x Height*CellHeight)
}

// NewGraphicsCanvas creates a new pixel canvas at the given cell position
func NewGraphicsCanvas(x, y, width, height int) *GraphicsCanvas {
	pixelWidth := width * renderer.CellWidth
	pixelHeight := height * renderer.CellHeight
	return &GraphicsCanvas{
		X:      x,
		Y:      y,
		Width:  width,
		Height: height,
		image:  ebiten.NewImage(pixelWidth, pixelHeight),
	}
}

// Clear fills the canvas with the background color
func (c *GraphicsCanvas) Clear(bg color.RGBA) {
	c.image.Fill(bg)
}

// Image returns the underlying ebiten.Image for compositing
func (c *GraphicsCanvas) Image() *ebiten.Image {
	return c.image
}

// PixelPosition returns the screen pixel coordinates for compositing
func (c *GraphicsCanvas) PixelPosition() (int, int) {
	return renderer.PaddingX + c.X*renderer.CellWidth,
		renderer.PaddingY + c.Y*renderer.CellHeight
}

// PixelSize returns the canvas size in pixels
func (c *GraphicsCanvas) PixelSize() (int, int) {
	return c.Width * renderer.CellWidth, c.Height * renderer.CellHeight
}

// DrawCircle draws a filled circle at the given pixel coordinates
func (c *GraphicsCanvas) DrawCircle(cx, cy, radius float64, clr color.RGBA) {
	if radius < 0.5 {
		// Sub-pixel: just draw a single pixel
		c.setPixel(int(cx), int(cy), clr)
		return
	}

	// Rasterize filled circle using midpoint algorithm
	r := int(radius + 0.5)
	x0, y0 := int(cx), int(cy)

	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx*dx+dy*dy <= r*r {
				c.setPixel(x0+dx, y0+dy, clr)
			}
		}
	}
}

// DrawGlow draws a radial gradient glow effect for stars
func (c *GraphicsCanvas) DrawGlow(cx, cy, innerRadius, outerRadius float64, clr color.RGBA) {
	r := int(outerRadius + 0.5)
	x0, y0 := int(cx), int(cy)

	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			dist := math.Sqrt(float64(dx*dx + dy*dy))
			if dist > outerRadius {
				continue
			}
			if dist < innerRadius {
				// Inside core - full color handled by DrawCircle
				continue
			}

			// Interpolate alpha based on distance from inner to outer radius
			t := (dist - innerRadius) / (outerRadius - innerRadius)
			alpha := uint8(float64(clr.A) * (1.0 - t*t)) // Quadratic falloff

			if alpha > 0 {
				glowClr := color.RGBA{clr.R, clr.G, clr.B, alpha}
				c.blendPixel(x0+dx, y0+dy, glowClr)
			}
		}
	}
}

// setPixel sets a pixel, checking bounds
func (c *GraphicsCanvas) setPixel(x, y int, clr color.RGBA) {
	w, h := c.PixelSize()
	if x >= 0 && x < w && y >= 0 && y < h {
		c.image.Set(x, y, clr)
	}
}

// blendPixel blends a pixel with alpha compositing
func (c *GraphicsCanvas) blendPixel(x, y int, clr color.RGBA) {
	w, h := c.PixelSize()
	if x < 0 || x >= w || y < 0 || y >= h {
		return
	}

	// Get existing pixel
	existing := c.image.At(x, y)
	r, g, b, a := existing.RGBA()
	// Convert from 0-65535 to 0-255
	er, eg, eb, ea := uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)

	// Alpha blend
	srcA := float64(clr.A) / 255.0
	dstA := float64(ea) / 255.0
	outA := srcA + dstA*(1-srcA)

	if outA > 0 {
		nr := uint8((float64(clr.R)*srcA + float64(er)*dstA*(1-srcA)) / outA)
		ng := uint8((float64(clr.G)*srcA + float64(eg)*dstA*(1-srcA)) / outA)
		nb := uint8((float64(clr.B)*srcA + float64(eb)*dstA*(1-srcA)) / outA)
		na := uint8(outA * 255)
		c.image.Set(x, y, color.RGBA{nr, ng, nb, na})
	}
}
