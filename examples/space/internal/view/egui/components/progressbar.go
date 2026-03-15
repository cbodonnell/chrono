package components

import (
	"fmt"
	"image/color"

	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/colors"
	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/renderer"
)

// ProgressBar displays a visual progress bar
type ProgressBar struct {
	X, Y     int
	BarWidth int
	Current  int
	Max      int
	Label    string

	ShowValues bool
	FillChar   rune
	EmptyChar  rune
	FillColor  color.RGBA
	EmptyColor color.RGBA
	LabelColor color.RGBA
	ValueColor color.RGBA
}

// NewProgressBar creates a new progress bar
func NewProgressBar(x, y, width int) *ProgressBar {
	return &ProgressBar{
		X:          x,
		Y:          y,
		BarWidth:   width,
		Current:    0,
		Max:        100,
		ShowValues: true,
		FillChar:   '█',
		EmptyChar:  '░',
		FillColor:  colors.Primary,
		EmptyColor: colors.Muted,
		LabelColor: colors.White,
		ValueColor: colors.White,
	}
}

// WithLabel sets the label shown before the bar
func (p *ProgressBar) WithLabel(label string) *ProgressBar {
	p.Label = label
	return p
}

// WithValues sets current and max values
func (p *ProgressBar) WithValues(current, max int) *ProgressBar {
	p.Current = current
	p.Max = max
	return p
}

// Percent returns the current percentage (0.0 to 1.0)
func (p *ProgressBar) Percent() float64 {
	if p.Max <= 0 {
		return 0
	}
	pct := float64(p.Current) / float64(p.Max)
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}
	return pct
}

// Draw renders the progress bar to the terminal
func (p *ProgressBar) Draw(term *renderer.Terminal) {
	x := p.X
	y := p.Y

	if p.Label != "" {
		term.Print(x, y, p.Label+" ", p.LabelColor)
		x += len(p.Label) + 1
	}

	barWidth := p.BarWidth
	if p.Label != "" {
		barWidth -= len(p.Label) + 1
	}

	valuesText := ""
	if p.ShowValues {
		maxDigits := len(fmt.Sprintf("%d", p.Max))
		valuesText = fmt.Sprintf(" %*d/%d", maxDigits, p.Current, p.Max)
		barWidth -= len(valuesText)
	}

	if barWidth < 1 {
		barWidth = 1
	}

	fillWidth := int(p.Percent() * float64(barWidth))
	if fillWidth > barWidth {
		fillWidth = barWidth
	}
	if fillWidth == 0 && p.Percent() > 0 {
		fillWidth = 1
	}

	for i := 0; i < barWidth; i++ {
		if i < fillWidth {
			term.SetCharWithColor(x+i, y, p.FillChar, p.FillColor)
		} else {
			term.SetCharWithColor(x+i, y, p.EmptyChar, p.EmptyColor)
		}
	}

	if p.ShowValues {
		term.Print(x+barWidth, y, valuesText, p.ValueColor)
	}
}
