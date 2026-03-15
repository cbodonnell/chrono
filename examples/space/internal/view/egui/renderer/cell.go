package renderer

import "image/color"

// Cell represents a single character cell in the terminal grid
type Cell struct {
	Char       rune
	Foreground color.RGBA
	Background color.RGBA
	Bold       bool
	Underline  bool
}

// DefaultCell returns a cell with default styling
func DefaultCell() Cell {
	return Cell{
		Char:       ' ',
		Foreground: color.RGBA{0xff, 0xff, 0xff, 0xff},
		Background: color.RGBA{0x00, 0x00, 0x00, 0x00},
		Bold:       false,
		Underline:  false,
	}
}
