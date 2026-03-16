package physics

// Body represents a celestial body in the simulation
type Body struct {
	ID     string
	Kind   string  // "star", "planet", "asteroid", "debris"
	Name   string
	Mass   float64 // Actual mass (gravitational influence)
	Radius float64 // For collision detection
	X, Y   float64 // Position
	VX, VY float64 // Velocity
	AX, AY float64 // Acceleration (computed each step)
	Alive  bool
}

// KineticEnergy returns the kinetic energy of the body
func (b *Body) KineticEnergy() float64 {
	return 0.5 * b.Mass * (b.VX*b.VX + b.VY*b.VY)
}

// Momentum returns the momentum vector of the body
func (b *Body) Momentum() (px, py float64) {
	return b.Mass * b.VX, b.Mass * b.VY
}
