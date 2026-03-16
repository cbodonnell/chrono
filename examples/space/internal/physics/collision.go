package physics

import "math"

// CollisionType indicates what kind of collision occurred
type CollisionType string

const (
	CollisionAbsorption  CollisionType = "absorption"  // Larger body absorbs smaller
	CollisionDestruction CollisionType = "destruction" // Both bodies destroyed, debris created
)

// CollisionEvent describes a collision that occurred
type CollisionEvent struct {
	Type       CollisionType
	Body1ID    string    // First body involved
	Body2ID    string    // Second body involved
	SurvivorID string    // ID of survivor (empty if both destroyed)
	DebrisIDs  []string  // IDs of any debris created
	X, Y       float64   // Collision position
	Debris     []*Body   // The actual debris bodies created
}

// collisionPair tracks two bodies that are colliding
type collisionPair struct {
	b1, b2   *Body
	distance float64
}

// handleCollisions detects and resolves all collisions
// Returns events describing what happened
func (s *Simulation) handleCollisions() []CollisionEvent {
	var events []CollisionEvent

	// Find all colliding pairs
	pairs := s.findCollisions()

	// Sort by distance (handle closest collisions first)
	// Simple insertion sort since we expect few collisions per tick
	for i := 1; i < len(pairs); i++ {
		for j := i; j > 0 && pairs[j].distance < pairs[j-1].distance; j-- {
			pairs[j], pairs[j-1] = pairs[j-1], pairs[j]
		}
	}

	// Handle each collision
	for _, pair := range pairs {
		// Skip if either body was already destroyed in a previous collision this tick
		if !pair.b1.Alive || !pair.b2.Alive {
			continue
		}

		event := s.resolveCollision(pair.b1, pair.b2)
		events = append(events, event)
	}

	return events
}

// findCollisions returns all pairs of bodies that are overlapping
func (s *Simulation) findCollisions() []collisionPair {
	var pairs []collisionPair

	for i, b1 := range s.Bodies {
		if !b1.Alive {
			continue
		}
		for j := i + 1; j < len(s.Bodies); j++ {
			b2 := s.Bodies[j]
			if !b2.Alive {
				continue
			}

			dx := b1.X - b2.X
			dy := b1.Y - b2.Y
			dist := math.Sqrt(dx*dx + dy*dy)
			threshold := b1.Radius + b2.Radius

			if dist < threshold {
				pairs = append(pairs, collisionPair{b1: b1, b2: b2, distance: dist})
			}
		}
	}

	return pairs
}

// resolveCollision handles a collision between two bodies with momentum conservation
func (s *Simulation) resolveCollision(b1, b2 *Body) CollisionEvent {
	// Collision position (mass-weighted center)
	totalMass := b1.Mass + b2.Mass
	cx := (b1.X*b1.Mass + b2.X*b2.Mass) / totalMass
	cy := (b1.Y*b1.Mass + b2.Y*b2.Mass) / totalMass

	// Determine larger and smaller by mass
	larger, smaller := b1, b2
	if b2.Mass > b1.Mass {
		larger, smaller = b2, b1
	}

	massRatio := larger.Mass / smaller.Mass

	// Combined momentum
	px := b1.Mass*b1.VX + b2.Mass*b2.VX
	py := b1.Mass*b1.VY + b2.Mass*b2.VY

	if massRatio >= 3.0 {
		// Absorption: larger body absorbs smaller
		return s.handleAbsorption(larger, smaller, px, py, cx, cy)
	}

	// Destruction: both destroyed, debris created
	return s.handleDestruction(b1, b2, px, py, cx, cy)
}

// handleAbsorption processes an absorption collision
func (s *Simulation) handleAbsorption(larger, smaller *Body, px, py, cx, cy float64) CollisionEvent {
	// Destroy smaller body
	smaller.Alive = false

	// Merge compositions (gas dominates rock)
	larger.Composition = MergeComposition(larger.Composition, smaller.Composition, larger.Mass, smaller.Mass)

	// Larger body gains mass and inherits combined momentum
	larger.Mass += smaller.Mass
	larger.VX = px / larger.Mass
	larger.VY = py / larger.Mass

	// Grow radius slightly (volume-based growth would be cube root, but we simplify)
	larger.Radius += smaller.Radius * 0.25

	return CollisionEvent{
		Type:       CollisionAbsorption,
		Body1ID:    larger.ID,
		Body2ID:    smaller.ID,
		SurvivorID: larger.ID,
		X:          cx,
		Y:          cy,
	}
}

// handleDestruction processes a mutual destruction collision
func (s *Simulation) handleDestruction(b1, b2 *Body, px, py, cx, cy float64) CollisionEvent {
	// Both bodies destroyed
	b1.Alive = false
	b2.Alive = false

	// Determine resulting composition (gas dominates)
	resultComp := MergeComposition(b1.Composition, b2.Composition, b1.Mass, b2.Mass)

	// Create 2-4 pieces that share the momentum
	numPieces := 2 + int(math.Min(float64(int((b1.Mass+b2.Mass)/5)), 2))
	totalMass := b1.Mass + b2.Mass
	pieceMass := totalMass / float64(numPieces)
	pieceRadius := math.Max(0.2, (b1.Radius+b2.Radius)*0.2)

	var pieces []*Body
	var pieceIDs []string

	// Distribute momentum among pieces with some spread
	for i := 0; i < numPieces; i++ {
		// Base velocity from momentum conservation
		baseVX := px / totalMass
		baseVY := py / totalMass

		// Add some angular spread
		angle := float64(i) * 2 * math.Pi / float64(numPieces)
		spreadSpeed := math.Sqrt(baseVX*baseVX+baseVY*baseVY) * 0.3

		d := &Body{
			Composition: resultComp,
			Mass:        pieceMass,
			Radius:      pieceRadius,
			X:           cx + pieceRadius*2*math.Cos(angle),
			Y:           cy + pieceRadius*2*math.Sin(angle),
			VX:          baseVX + spreadSpeed*math.Cos(angle),
			VY:          baseVY + spreadSpeed*math.Sin(angle),
			Alive:       true,
		}
		// Kind is derived from mass + composition automatically
		pieces = append(pieces, d)
		s.Bodies = append(s.Bodies, d)
	}

	// Collect IDs (they need to be assigned by the caller)
	for _, d := range pieces {
		pieceIDs = append(pieceIDs, d.ID)
	}

	return CollisionEvent{
		Type:      CollisionDestruction,
		Body1ID:   b1.ID,
		Body2ID:   b2.ID,
		X:         cx,
		Y:         cy,
		DebrisIDs: pieceIDs,
		Debris:    pieces,
	}
}
