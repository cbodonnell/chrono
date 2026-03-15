package components

import (
	"image/color"

	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/colors"
	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/renderer"
)

// FooterAction represents a key binding shown in the footer
type FooterAction struct {
	Key         string
	Description string
}

// Footer displays context-sensitive help text at the bottom of the screen
type Footer struct {
	X, Y     int
	MaxWidth int
	Actions  []FooterAction

	KeyColor         color.RGBA
	DescriptionColor color.RGBA
	Separator        string
}

// NewFooter creates a new footer
func NewFooter(x, y, maxWidth int) *Footer {
	return &Footer{
		X:                x,
		Y:                y,
		MaxWidth:         maxWidth,
		Actions:          []FooterAction{},
		KeyColor:         colors.Primary,
		DescriptionColor: colors.Muted,
		Separator:        "  ",
	}
}

// AddAction adds an action to the footer
func (f *Footer) AddAction(key, description string) *Footer {
	f.Actions = append(f.Actions, FooterAction{Key: key, Description: description})
	return f
}

// Draw renders the footer to the terminal
func (f *Footer) Draw(term *renderer.Terminal) {
	x := f.X

	for i, action := range f.Actions {
		if i > 0 {
			term.Print(x, f.Y, f.Separator, f.DescriptionColor)
			x += len(f.Separator)
		}

		actionText := "[" + action.Key + "] " + action.Description
		if x+len(actionText) > f.X+f.MaxWidth {
			break
		}

		term.Print(x, f.Y, "[", f.DescriptionColor)
		x++
		term.Print(x, f.Y, action.Key, f.KeyColor)
		x += len(action.Key)
		term.Print(x, f.Y, "] ", f.DescriptionColor)
		x += 2
		term.Print(x, f.Y, action.Description, f.DescriptionColor)
		x += len(action.Description)
	}
}
