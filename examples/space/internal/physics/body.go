package physics

// Composition types
const (
	CompGas  = "gas"  // Hydrogen/helium - can fuse if massive enough
	CompRite = "rock" // Silicates and metals - can never become a star
)

// Kind constants (derived from mass + composition)
const (
	KindStar     = "star"     // Gas, mass >= 500 (fusion)
	KindGiant    = "giant"    // Gas, mass 50-500 (gas giant / brown dwarf)
	KindCloud    = "cloud"    // Gas, mass < 50 (gas remnant)
	KindPlanet   = "planet"   // Rock, mass >= 15 (spherical)
	KindAsteroid = "asteroid" // Rock, mass 1.5-15 (irregular)
	KindDebris   = "debris"   // Rock, mass < 1.5 (fragments)
)

// Mass thresholds for classification
const (
	MassFusion    = 500.0 // Minimum mass for hydrogen fusion (star)
	MassSpherical = 15.0  // Minimum mass for self-gravity to create sphere
	MassAsteroid  = 1.5   // Minimum mass for asteroid (vs debris)
	MassCoalesce  = 3.0   // Below this, collisions coalesce rather than shatter
)

// Body represents a celestial body in the simulation
type Body struct {
	ID          string
	Name        string
	Composition string  // "gas" or "rock"
	Mass        float64 // Actual mass (gravitational influence)
	Radius      float64 // For collision detection
	X, Y        float64 // Position
	VX, VY      float64 // Velocity
	AX, AY      float64 // Acceleration (computed each step)
	Alive       bool
}

// Kind returns the derived classification based on mass and composition
func (b *Body) Kind() string {
	return DeriveKind(b.Mass, b.Composition)
}

// DeriveKind determines body classification from mass and composition
func DeriveKind(mass float64, composition string) string {
	if composition == CompGas {
		if mass >= MassFusion {
			return KindStar
		}
		if mass >= MassSpherical {
			return KindGiant
		}
		return KindCloud
	}
	// Rock composition
	if mass >= MassSpherical {
		return KindPlanet
	}
	if mass >= MassAsteroid {
		return KindAsteroid
	}
	return KindDebris
}

// KineticEnergy returns the kinetic energy of the body
func (b *Body) KineticEnergy() float64 {
	return 0.5 * b.Mass * (b.VX*b.VX + b.VY*b.VY)
}

// Momentum returns the momentum vector of the body
func (b *Body) Momentum() (px, py float64) {
	return b.Mass * b.VX, b.Mass * b.VY
}

// MergeComposition determines the resulting composition when two bodies merge
// Gas dominates rock (rock vaporizes or sinks to core)
func MergeComposition(comp1, comp2 string, mass1, mass2 float64) string {
	if comp1 == CompGas || comp2 == CompGas {
		return CompGas
	}
	return CompRite
}
