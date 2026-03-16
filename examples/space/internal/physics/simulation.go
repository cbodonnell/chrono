package physics

import "math"

// Config holds simulation parameters
type Config struct {
	G                float64 // Gravitational constant
	SofteningEpsilon float64 // Prevents singularities at close range
	UniverseSize     float64 // Boundary size (0 to UniverseSize)
}

// DefaultConfig returns sensible defaults for the simulation
func DefaultConfig() Config {
	return Config{
		G:                1.0,
		SofteningEpsilon: 0.5,
		UniverseSize:     100.0,
	}
}

// Simulation manages the physics of all bodies
type Simulation struct {
	Bodies []*Body
	Config Config
}

// NewSimulation creates a new simulation with the given config
func NewSimulation(config Config) *Simulation {
	return &Simulation{
		Bodies: make([]*Body, 0),
		Config: config,
	}
}

// AddBody adds a body to the simulation
func (s *Simulation) AddBody(b *Body) {
	s.Bodies = append(s.Bodies, b)
}

// Step advances the simulation by dt seconds
// Returns collision events that occurred during this step
func (s *Simulation) Step(dt float64) []CollisionEvent {
	// 1. Reset accelerations
	for _, b := range s.Bodies {
		if !b.Alive {
			continue
		}
		b.AX = 0
		b.AY = 0
	}

	// 2. Compute gravitational accelerations
	s.computeGravity()

	// 3. Symplectic Euler integration
	// Update velocities first (using current acceleration)
	for _, b := range s.Bodies {
		if !b.Alive {
			continue
		}
		b.VX += b.AX * dt
		b.VY += b.AY * dt
	}

	// Update positions (using updated velocity)
	for _, b := range s.Bodies {
		if !b.Alive {
			continue
		}
		b.X += b.VX * dt
		b.Y += b.VY * dt
	}

	// 4. Handle boundary conditions (soft bounce)
	s.handleBoundaries()

	// 5. Detect and handle collisions
	events := s.handleCollisions()

	return events
}

// computeGravity calculates gravitational forces between all body pairs
func (s *Simulation) computeGravity() {
	G := s.Config.G
	eps2 := s.Config.SofteningEpsilon * s.Config.SofteningEpsilon

	for i, b1 := range s.Bodies {
		if !b1.Alive {
			continue
		}
		for j := i + 1; j < len(s.Bodies); j++ {
			b2 := s.Bodies[j]
			if !b2.Alive {
				continue
			}

			// Vector from b1 to b2
			dx := b2.X - b1.X
			dy := b2.Y - b1.Y

			// Distance squared with softening
			r2 := dx*dx + dy*dy + eps2
			r := math.Sqrt(r2)

			// Force magnitude: F = G * m1 * m2 / r²
			// Acceleration on b1: a1 = F / m1 = G * m2 / r²
			// Acceleration on b2: a2 = F / m2 = G * m1 / r²

			// Unit vector from b1 to b2
			ux := dx / r
			uy := dy / r

			// Acceleration magnitudes
			a1 := G * b2.Mass / r2
			a2 := G * b1.Mass / r2

			// Apply accelerations (opposite directions)
			b1.AX += a1 * ux
			b1.AY += a1 * uy
			b2.AX -= a2 * ux
			b2.AY -= a2 * uy
		}
	}
}

// handleBoundaries keeps bodies within the universe bounds
func (s *Simulation) handleBoundaries() {
	size := s.Config.UniverseSize

	for _, b := range s.Bodies {
		if !b.Alive {
			continue
		}

		// Bounce off walls with some energy loss
		if b.X < 0 {
			b.X = 0
			b.VX = -b.VX * 0.8
		} else if b.X > size {
			b.X = size
			b.VX = -b.VX * 0.8
		}

		if b.Y < 0 {
			b.Y = 0
			b.VY = -b.VY * 0.8
		} else if b.Y > size {
			b.Y = size
			b.VY = -b.VY * 0.8
		}
	}
}

// GetBody returns a body by ID, or nil if not found
func (s *Simulation) GetBody(id string) *Body {
	for _, b := range s.Bodies {
		if b.ID == id {
			return b
		}
	}
	return nil
}

// AliveBodies returns all living bodies
func (s *Simulation) AliveBodies() []*Body {
	alive := make([]*Body, 0)
	for _, b := range s.Bodies {
		if b.Alive {
			alive = append(alive, b)
		}
	}
	return alive
}
