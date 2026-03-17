package renderer

import (
	"image"
	"image/color"

	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/assets/fonts"
	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/colors"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/zachomedia/go-bdf"
)

const (
	// Grid dimensions
	Cols = colors.ContentWidth  // 80
	Rows = colors.ContentHeight // 30

	// Cell dimensions in pixels (matched to Cozette 6x13)
	CellWidth  = 6
	CellHeight = 13

	// Padding around the grid
	PaddingX = 20
	PaddingY = 20

	// Scale factor for window size
	Scale = 1.5
)

// ScreenWidth returns the logical screen width in pixels
func ScreenWidth() int {
	return Cols*CellWidth + 2*PaddingX
}

// ScreenHeight returns the logical screen height in pixels
func ScreenHeight() int {
	return Rows*CellHeight + 2*PaddingY
}

// WindowWidth returns the window width (scaled)
func WindowWidth() int {
	return int(float64(ScreenWidth()) * Scale)
}

// WindowHeight returns the window height (scaled)
func WindowHeight() int {
	return int(float64(ScreenHeight()) * Scale)
}

// CompositeImage represents an image to overlay after cell rendering
type CompositeImage struct {
	Image *ebiten.Image
	X, Y  int // Screen pixel coordinates
}

// Terminal represents the terminal rendering surface
type Terminal struct {
	cells      [][]Cell
	face       text.Face
	dirty      bool
	composites []*CompositeImage // Images to overlay after cell rendering
}

// NewTerminal creates a new terminal renderer
func NewTerminal() (*Terminal, error) {
	bdfFont, err := bdf.Parse(fonts.CozetteBDF)
	if err != nil {
		return nil, err
	}

	fontFace := bdfFont.NewFace()
	face := text.NewGoXFace(fontFace)

	t := &Terminal{
		cells: make([][]Cell, Rows),
		face:  face,
		dirty: true,
	}

	for y := 0; y < Rows; y++ {
		t.cells[y] = make([]Cell, Cols)
		for x := 0; x < Cols; x++ {
			t.cells[y][x] = DefaultCell()
		}
	}

	return t, nil
}

// Clear resets all cells to empty
func (t *Terminal) Clear() {
	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			t.cells[y][x] = DefaultCell()
		}
	}
	t.dirty = true
}

// RegisterComposite queues an image to be drawn after cell rendering
func (t *Terminal) RegisterComposite(img *ebiten.Image, x, y int) {
	t.composites = append(t.composites, &CompositeImage{
		Image: img,
		X:     x,
		Y:     y,
	})
}

// ClearComposites clears the composite queue (call each frame)
func (t *Terminal) ClearComposites() {
	t.composites = t.composites[:0]
}

// SetCell sets a single cell at the given position
func (t *Terminal) SetCell(x, y int, cell Cell) {
	if x >= 0 && x < Cols && y >= 0 && y < Rows {
		t.cells[y][x] = cell
		t.dirty = true
	}
}

// SetChar sets a character at the given position with default colors
func (t *Terminal) SetChar(x, y int, ch rune) {
	t.SetCell(x, y, Cell{
		Char:       ch,
		Foreground: colors.White,
		Background: color.RGBA{0, 0, 0, 0},
	})
}

// SetCharWithColor sets a character with a specific foreground color
func (t *Terminal) SetCharWithColor(x, y int, ch rune, fg color.RGBA) {
	t.SetCell(x, y, Cell{
		Char:       ch,
		Foreground: fg,
		Background: color.RGBA{0, 0, 0, 0},
	})
}

// Print writes a string starting at the given position
func (t *Terminal) Print(x, y int, text string, fg color.RGBA) {
	for i, ch := range text {
		if x+i >= Cols {
			break
		}
		t.SetCharWithColor(x+i, y, ch, fg)
	}
}

// PrintCentered prints text centered on the given row
func (t *Terminal) PrintCentered(y int, text string, fg color.RGBA) {
	x := (Cols - len(text)) / 2
	if x < 0 {
		x = 0
	}
	t.Print(x, y, text, fg)
}

// DrawBox draws a box with rounded corners
func (t *Terminal) DrawBox(x, y, width, height int, fg color.RGBA) {
	if width < 2 || height < 2 {
		return
	}

	const (
		topLeft     = '╭'
		topRight    = '╮'
		bottomLeft  = '╰'
		bottomRight = '╯'
		horizontal  = '─'
		vertical    = '│'
	)

	t.SetCharWithColor(x, y, topLeft, fg)
	t.SetCharWithColor(x+width-1, y, topRight, fg)
	t.SetCharWithColor(x, y+height-1, bottomLeft, fg)
	t.SetCharWithColor(x+width-1, y+height-1, bottomRight, fg)

	for i := 1; i < width-1; i++ {
		t.SetCharWithColor(x+i, y, horizontal, fg)
		t.SetCharWithColor(x+i, y+height-1, horizontal, fg)
	}

	for i := 1; i < height-1; i++ {
		t.SetCharWithColor(x, y+i, vertical, fg)
		t.SetCharWithColor(x+width-1, y+i, vertical, fg)
	}
}

// DrawBoxWithTitle draws a box with a title in the top border
func (t *Terminal) DrawBoxWithTitle(x, y, width, height int, title string, fg color.RGBA) {
	t.DrawBox(x, y, width, height, fg)
	if title != "" && width > 4 {
		titleX := x + 2
		titleText := " " + title + " "
		if len(titleText) > width-4 {
			titleText = titleText[:width-4]
		}
		t.Print(titleX, y, titleText, fg)
	}
}

// Draw renders the terminal to the screen
func (t *Terminal) Draw(screen *ebiten.Image) {
	screen.Fill(colors.Background)

	for y := 0; y < Rows; y++ {
		for x := 0; x < Cols; x++ {
			cell := t.cells[y][x]
			if cell.Char == ' ' && cell.Background.A == 0 {
				continue
			}

			px := float64(PaddingX + x*CellWidth)
			py := float64(PaddingY + y*CellHeight)

			if cell.Background.A > 0 {
				bgRect := image.Rect(
					int(px), int(py),
					int(px)+CellWidth, int(py)+CellHeight,
				)
				subImg := screen.SubImage(bgRect).(*ebiten.Image)
				subImg.Fill(cell.Background)
			}

			if cell.Char != ' ' {
				op := &text.DrawOptions{}
				op.GeoM.Translate(px, py)
				op.ColorScale.ScaleWithColor(cell.Foreground)
				op.Filter = ebiten.FilterNearest
				text.Draw(screen, string(cell.Char), t.face, op)
			}

			if cell.Underline {
				underlineY := int(py) + CellHeight - 2
				for i := 0; i < CellWidth; i++ {
					screen.Set(int(px)+i, underlineY, cell.Foreground)
				}
			}
		}
	}

	// Draw composited images (like the universe canvas) on top
	for _, comp := range t.composites {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(comp.X), float64(comp.Y))
		screen.DrawImage(comp.Image, op)
	}

	t.dirty = false
}
