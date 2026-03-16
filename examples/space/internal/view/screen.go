package view

import (
	"fmt"
	"image/color"
	"time"

	"github.com/cbodonnell/chrono/examples/space/internal/generate"
	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/colors"
	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/components"
	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/input"
	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/renderer"
	"github.com/cbodonnell/chrono/examples/space/internal/view/egui/screen"
	"github.com/cbodonnell/chrono/pkg/entity"
	"github.com/cbodonnell/chrono/pkg/store"
	"github.com/hajimehoshi/ebiten/v2"
)

// Step sizes for time scrubbing
var stepSizes = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	15 * time.Minute,
	1 * time.Hour,
}

// Screen is the main viewer screen
type Screen struct {
	store *store.EntityStore

	// Time navigation
	currentTime time.Time
	startTime   time.Time
	endTime     time.Time
	stepIndex   int

	// Playback
	playing  bool
	playTick int // frame counter for playback speed

	// Event selection
	selectedEventIndex int

	// Cached state
	masses []*entity.Entity
	events []*entity.Entity

	// UI components
	footer      *components.Footer
	progressBar *components.ProgressBar
	grid        *Grid
}

// NewScreen creates a new viewer screen
func NewScreen(es *store.EntityStore) *Screen {
	// Time range matches generate constants
	now := time.Now()
	startTime := now.Add(-time.Hour).Truncate(time.Minute)
	endTime := startTime.Add(time.Duration(generate.TickCount) * generate.TickDuration)

	s := &Screen{
		store:       es,
		currentTime: startTime,
		startTime:   startTime,
		endTime:     endTime,
		stepIndex:   0, // Default to 1 minute steps
		footer:      components.NewFooter(0, renderer.Rows-1, renderer.Cols),
		progressBar: components.NewProgressBar(3, renderer.Rows-4, renderer.Cols-6),
		grid:        NewGrid(3, 3, 58, 19),
	}

	s.footer.
		AddAction("Space", "Play/Pause").
		AddAction("\u2190/\u2192", "Scrub").
		AddAction("\u2191/\u2193", "Events").
		AddAction("S", "Step").
		AddAction("Q", "Quit")

	s.progressBar.ShowValues = true
	s.progressBar.FillColor = colors.Primary

	// Initial query
	s.refresh()

	return s
}

// Update handles input and returns the next screen state
func (s *Screen) Update(inp *input.Handler) screen.State {
	key := inp.GetPressedKey()

	// Handle held keys for continuous scrubbing
	if inp.KeyRepeat(ebiten.KeyLeft, 30, 5) {
		s.scrubTime(-1)
	} else if inp.KeyRepeat(ebiten.KeyRight, 30, 5) {
		s.scrubTime(1)
	}

	switch key {
	case input.KeyQuit:
		return screen.StateQuit
	case input.KeyLeft:
		s.playing = false
		s.scrubTime(-1)
	case input.KeyRight:
		s.playing = false
		s.scrubTime(1)
	case input.KeyUp:
		s.selectEvent(-1)
	case input.KeyDown:
		s.selectEvent(1)
	case input.KeyS:
		s.cycleStepSize(1)
	case input.KeySpace:
		s.playing = !s.playing
	}

	// Auto-advance when playing (every 6 frames = ~10 steps/sec at 60fps)
	if s.playing {
		s.playTick++
		if s.playTick >= 6 {
			s.playTick = 0
			s.scrubTime(1)
			// Stop at end
			if s.currentTime.Equal(s.endTime) || s.currentTime.After(s.endTime) {
				s.playing = false
			}
		}
	}

	return screen.StateView
}

func (s *Screen) scrubTime(direction int) {
	step := stepSizes[s.stepIndex]
	newTime := s.currentTime.Add(time.Duration(direction) * step)

	// Clamp to valid range
	if newTime.Before(s.startTime) {
		newTime = s.startTime
	}
	if newTime.After(s.endTime) {
		newTime = s.endTime
	}

	if newTime != s.currentTime {
		s.currentTime = newTime
		s.refresh()
	}
}

func (s *Screen) cycleStepSize(direction int) {
	s.stepIndex += direction
	if s.stepIndex < 0 {
		s.stepIndex = len(stepSizes) - 1
	}
	if s.stepIndex >= len(stepSizes) {
		s.stepIndex = 0
	}
}

func (s *Screen) selectEvent(direction int) {
	if len(s.events) == 0 {
		return
	}

	s.selectedEventIndex += direction
	if s.selectedEventIndex < 0 {
		s.selectedEventIndex = 0
	}
	if s.selectedEventIndex >= len(s.events) {
		s.selectedEventIndex = len(s.events) - 1
	}
}

func (s *Screen) refresh() {
	// Query masses at current time point
	masses, err := s.store.Query(&store.Query{
		EntityType: "mass",
		TimeRange: &store.TimeRange{
			From: 0,
			To:   s.currentTime.UnixNano(),
		},
	})
	if err == nil {
		s.masses = masses
	}

	// Query recent events (last 10 minutes from current time, oldest first)
	eventStart := s.currentTime.Add(-10 * time.Minute)
	events, err := s.store.Query(&store.Query{
		EntityType: "event",
		TimeRange: &store.TimeRange{
			From: eventStart.UnixNano(),
			To:   s.currentTime.UnixNano(),
		},
		AllVersions: true,
		Limit:       10,
	})
	if err == nil {
		s.events = events
	}
}

// Draw renders the screen to the terminal
func (s *Screen) Draw(term *renderer.Terminal) {
	// Draw outer box
	term.DrawBoxWithTitle(0, 0, renderer.Cols, renderer.Rows, "/space", colors.Muted)

	// Draw universe grid
	s.grid.Draw(term, s.masses)

	// Draw legend below universe grid
	s.drawLegend(term)

	// Draw events panel
	s.drawEventsPanel(term)

	// Draw time info
	s.drawTimeInfo(term)

	// Draw footer
	s.footer.Draw(term)
}

func (s *Screen) drawEventsPanel(term *renderer.Terminal) {
	// Events list panel at top right
	panelX := 63
	panelY := 2
	panelWidth := 16
	listHeight := 12

	term.DrawBoxWithTitle(panelX, panelY, panelWidth, listHeight, "Events", colors.Muted)

	// Draw events list
	y := panelY + 1
	maxEvents := listHeight - 2
	for i, evt := range s.events {
		if i >= maxEvents {
			break
		}

		eventType := evt.Fields["event_type"].S
		description := evt.Fields["description"].S

		// Truncate description to fit panel (leave room for icon and brackets)
		maxDescLen := panelWidth - 4
		if len(description) > maxDescLen {
			description = description[:maxDescLen-1]
		}

		// Event icon and color
		icon, clr := s.getEventAppearance(eventType)

		// Highlight selected event with brackets
		if i == s.selectedEventIndex {
			term.SetCharWithColor(panelX+1, y, '[', colors.Primary)
			term.SetCharWithColor(panelX+2, y, icon, clr)
			term.Print(panelX+4, y, description, colors.White)
			term.SetCharWithColor(panelX+panelWidth-2, y, ']', colors.Primary)
		} else {
			term.SetCharWithColor(panelX+1, y, icon, clr)
			term.Print(panelX+3, y, description, colors.Muted)
		}
		y++
	}

	// Details panel below events list
	s.drawEventDetails(term, panelX, panelY+listHeight, panelWidth)
}

func (s *Screen) drawEventDetails(term *renderer.Terminal, x, y, width int) {
	height := 10
	term.DrawBoxWithTitle(x, y, width, height, "Details", colors.Muted)

	// Show details for selected event
	if s.selectedEventIndex < 0 || s.selectedEventIndex >= len(s.events) {
		term.Print(x+1, y+1, "No selection", colors.Muted)
		return
	}

	evt := s.events[s.selectedEventIndex]
	eventType := evt.Fields["event_type"].S
	description := evt.Fields["description"].S
	ts := time.Unix(0, evt.Timestamp)

	// Event type with icon
	icon, clr := s.getEventAppearance(eventType)
	term.SetCharWithColor(x+1, y+1, icon, clr)
	term.Print(x+3, y+1, eventType, clr)

	// Timestamp
	term.Print(x+1, y+3, ts.Format("15:04:05"), colors.White)

	// Description (word-wrapped)
	contentWidth := width - 2
	line := y + 5
	for len(description) > 0 && line < y+height-1 {
		chunk := description
		if len(chunk) > contentWidth {
			chunk = description[:contentWidth]
		}
		term.Print(x+1, line, chunk, colors.Muted)
		description = description[len(chunk):]
		line++
	}
}

func (s *Screen) getEventAppearance(eventType string) (rune, color.RGBA) {
	switch eventType {
	case "formation":
		return '+', colors.Success
	case "absorption":
		return 'o', colors.Primary
	case "collision":
		return 'x', colors.Warning
	case "destruction":
		return '-', colors.Danger
	default:
		return '*', colors.Muted
	}
}

func (s *Screen) drawLegend(term *renderer.Terminal) {
	// Draw legend below the universe grid (two rows for 6 kinds)
	// All gray since actual colors are mass-based
	legendY := s.grid.Y + s.grid.Height + 1

	// Row 1: Gas bodies (radiating shapes) - ★ Star  ✦ Giant  * Cloud
	x := s.grid.X
	term.SetCharWithColor(x, legendY, '\u2605', colors.Muted) // ★
	term.Print(x+2, legendY, "Star", colors.Muted)

	x += 7
	term.SetCharWithColor(x, legendY, '\u2726', colors.Muted) // ✦
	term.Print(x+2, legendY, "Giant", colors.Muted)

	x += 8
	term.SetCharWithColor(x, legendY, '*', colors.Muted) // *
	term.Print(x+2, legendY, "Cloud", colors.Muted)

	// Row 2: Rock bodies (solid shapes) - ◉ Planet  • Asteroid  · Debris
	legendY++
	x = s.grid.X
	term.SetCharWithColor(x, legendY, '\u25C9', colors.Muted) // ◉
	term.Print(x+2, legendY, "Planet", colors.Muted)

	x += 9
	term.SetCharWithColor(x, legendY, '\u2022', colors.Muted) // •
	term.Print(x+2, legendY, "Asteroid", colors.Muted)

	x += 11
	term.SetCharWithColor(x, legendY, '\u00B7', colors.Muted) // ·
	term.Print(x+2, legendY, "Debris", colors.Muted)
}

func (s *Screen) drawTimeInfo(term *renderer.Terminal) {
	// Time display
	timeY := renderer.Rows - 4
	timeStr := s.currentTime.Format("15:04:05")

	// Draw play state and time
	if s.playing {
		term.Print(3, timeY, "\u25B6", colors.Success) // ▶ playing
	} else {
		term.Print(3, timeY, "\u2759\u2759", colors.Muted) // ❚❚ paused
	}
	term.Print(6, timeY, timeStr, colors.White)

	// Step size display
	stepStr := formatDuration(stepSizes[s.stepIndex])
	term.Print(50, timeY, fmt.Sprintf("Step: %s", stepStr), colors.Muted)

	// Progress bar showing position in timeline
	totalDuration := s.endTime.Sub(s.startTime)
	currentOffset := s.currentTime.Sub(s.startTime)
	tickNum := int(currentOffset / time.Minute)
	totalTicks := int(totalDuration / time.Minute)

	s.progressBar.WithValues(tickNum, totalTicks)
	s.progressBar.Y = renderer.Rows - 5
	s.progressBar.Draw(term)

	// Stats line - breakdown by type
	statsY := renderer.Rows - 3
	var stars, giants, clouds, planets, asteroids, debris int
	for _, m := range s.masses {
		if m.Fields["alive"].B {
			switch m.Fields["kind"].S {
			case "star":
				stars++
			case "giant":
				giants++
			case "cloud":
				clouds++
			case "planet":
				planets++
			case "asteroid":
				asteroids++
			case "debris":
				debris++
			}
		}
	}

	// Draw stats with icons: ★2 ✦1 *0 ●4 ◆12 ·3
	x := 3

	// Gas bodies (radiating shapes)
	term.SetCharWithColor(x, statsY, '\u2605', colors.MassExtreme) // ★
	term.Print(x+1, statsY, fmt.Sprintf("%d", stars), colors.Muted)
	x += 4

	term.SetCharWithColor(x, statsY, '\u2726', colors.MassVeryHigh) // ✦
	term.Print(x+1, statsY, fmt.Sprintf("%d", giants), colors.Muted)
	x += 4

	term.SetCharWithColor(x, statsY, '*', colors.MassMedium) // *
	term.Print(x+1, statsY, fmt.Sprintf("%d", clouds), colors.Muted)
	x += 4

	// Rock bodies (solid shapes)
	term.SetCharWithColor(x, statsY, '\u25C9', colors.MassHigh) // ◉
	term.Print(x+1, statsY, fmt.Sprintf("%d", planets), colors.Muted)
	x += 4

	term.SetCharWithColor(x, statsY, '\u2022', colors.MassLow) // •
	term.Print(x+1, statsY, fmt.Sprintf("%d", asteroids), colors.Muted)
	x += 5

	term.SetCharWithColor(x, statsY, '\u00B7', colors.MassVeryLow) // ·
	term.Print(x+1, statsY, fmt.Sprintf("%d", debris), colors.Muted)
}

func formatDuration(d time.Duration) string {
	if d >= time.Minute {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%ds", int(d.Seconds()))
}
