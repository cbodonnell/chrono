package generate

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/cbodonnell/chrono/pkg/entity"
	"github.com/cbodonnell/chrono/pkg/store"
)

const (
	// Universe parameters
	universeSize = 100.0 // 0-100 coordinate space
	TickCount    = 600   // ticks (1 per minute)
	TickDuration = time.Minute
)

// Mass kinds
const (
	KindStar     = "star"
	KindPlanet   = "planet"
	KindAsteroid = "asteroid"
	KindDebris   = "debris"
)

// Event types
const (
	EventFormation   = "formation"
	EventCollision   = "collision"
	EventDestruction = "destruction"
	EventAbsorption  = "absorption"
)

// Mass represents a celestial body in the simulation
type Mass struct {
	ID     string
	Kind   string
	Name   string
	X, Y   float64
	VX, VY float64 // velocity (units per tick)
	Radius float64
	Alive  bool
	// For orbital motion
	OrbitCenterX float64
	OrbitCenterY float64
	OrbitRadius  float64
	OrbitSpeed   float64 // radians per tick
	OrbitAngle   float64
}

// Generator handles universe generation
type Generator struct {
	store   *store.EntityStore
	rng     *rand.Rand
	masses  []*Mass
	massID  int
	eventID int
}

// New creates a new generator
func New(es *store.EntityStore, seed int64) *Generator {
	return &Generator{
		store:  es,
		rng:    rand.New(rand.NewSource(seed)),
		masses: make([]*Mass, 0),
	}
}

// Generate creates a procedural universe with evolution over time
func (g *Generator) Generate() error {
	fmt.Println("Generating universe...")

	baseTime := time.Now().Add(-time.Hour).Truncate(time.Minute)

	// Create initial state at t=0
	if err := g.createInitialState(baseTime); err != nil {
		return fmt.Errorf("create initial state: %w", err)
	}
	fmt.Printf("Created %d initial masses\n", len(g.masses))

	// Evolve universe
	for tick := 1; tick < TickCount; tick++ {
		tickTime := baseTime.Add(time.Duration(tick) * TickDuration)
		if err := g.evolveTick(tick, tickTime); err != nil {
			return fmt.Errorf("evolve tick %d: %w", tick, err)
		}
	}

	fmt.Printf("Generation complete: %d events created\n", g.eventID)
	return nil
}

func (g *Generator) createInitialState(t time.Time) error {
	// Create 2 star systems
	numSystems := 2
	for i := 0; i < numSystems; i++ {
		if err := g.createStarSystem(t, i); err != nil {
			return err
		}
	}

	// Scatter 5-8 asteroids throughout
	numAsteroids := 5 + g.rng.Intn(4)
	for i := 0; i < numAsteroids; i++ {
		if err := g.createAsteroid(t, ""); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) createStarSystem(t time.Time, systemIndex int) error {
	// Position star
	starX := 15.0 + g.rng.Float64()*70.0
	starY := 15.0 + g.rng.Float64()*70.0

	// Create central star
	star := &Mass{
		ID:     g.nextMassID(),
		Kind:   KindStar,
		Name:   fmt.Sprintf("Sol-%d", systemIndex+1),
		X:      starX,
		Y:      starY,
		Radius: 3.0 + g.rng.Float64()*2.0,
		Alive:  true,
	}
	g.masses = append(g.masses, star)
	if err := g.writeMass(star, t); err != nil {
		return err
	}
	if err := g.writeEvent(EventFormation, fmt.Sprintf("Star %s ignited", star.Name), star.ID, t); err != nil {
		return err
	}

	// Create 2-3 planets orbiting the star
	numPlanets := 2 + g.rng.Intn(2)
	for i := 0; i < numPlanets; i++ {
		orbitRadius := 5.0 + float64(i)*4.0 + g.rng.Float64()*2.0
		orbitAngle := g.rng.Float64() * 2 * math.Pi
		orbitSpeed := (0.05 + g.rng.Float64()*0.1) / (1 + float64(i)*0.3) // Outer planets slower

		planet := &Mass{
			ID:           g.nextMassID(),
			Kind:         KindPlanet,
			Name:         fmt.Sprintf("%s-%c", star.Name, 'a'+rune(i)),
			X:            starX + orbitRadius*math.Cos(orbitAngle),
			Y:            starY + orbitRadius*math.Sin(orbitAngle),
			Radius:       1.0 + g.rng.Float64()*1.5,
			Alive:        true,
			OrbitCenterX: starX,
			OrbitCenterY: starY,
			OrbitRadius:  orbitRadius,
			OrbitSpeed:   orbitSpeed,
			OrbitAngle:   orbitAngle,
		}
		g.masses = append(g.masses, planet)
		if err := g.writeMass(planet, t); err != nil {
			return err
		}
		if err := g.writeEvent(EventFormation, fmt.Sprintf("Planet %s formed", planet.Name), planet.ID, t); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) createAsteroid(t time.Time, reason string) error {
	// Random velocity direction and speed (0.5-1.5 units per tick)
	angle := g.rng.Float64() * 2 * math.Pi
	speed := 0.5 + g.rng.Float64()

	asteroid := &Mass{
		ID:     g.nextMassID(),
		Kind:   KindAsteroid,
		Name:   fmt.Sprintf("Ast-%03d", g.massID),
		X:      g.rng.Float64() * universeSize,
		Y:      g.rng.Float64() * universeSize,
		VX:     speed * math.Cos(angle),
		VY:     speed * math.Sin(angle),
		Radius: 0.5 + g.rng.Float64()*0.5,
		Alive:  true,
	}
	g.masses = append(g.masses, asteroid)
	if err := g.writeMass(asteroid, t); err != nil {
		return err
	}

	desc := fmt.Sprintf("Asteroid %s detected", asteroid.Name)
	if reason != "" {
		desc = reason
	}
	return g.writeEvent(EventFormation, desc, asteroid.ID, t)
}

func (g *Generator) evolveTick(_ int, t time.Time) error {
	// Update positions for all alive masses
	for _, mass := range g.masses {
		if !mass.Alive {
			continue
		}

		// Planets orbit their stars
		if mass.Kind == KindPlanet && mass.OrbitRadius > 0 {
			mass.OrbitAngle += mass.OrbitSpeed
			mass.X = mass.OrbitCenterX + mass.OrbitRadius*math.Cos(mass.OrbitAngle)
			mass.Y = mass.OrbitCenterY + mass.OrbitRadius*math.Sin(mass.OrbitAngle)
			if err := g.writeMass(mass, t); err != nil {
				return err
			}
		} else if mass.Kind == KindAsteroid {
			// Asteroids move along their velocity vector
			mass.X += mass.VX
			mass.Y += mass.VY

			// Bounce off walls
			if mass.X < 0 {
				mass.X = 0
				mass.VX = -mass.VX
			} else if mass.X > universeSize {
				mass.X = universeSize
				mass.VX = -mass.VX
			}
			if mass.Y < 0 {
				mass.Y = 0
				mass.VY = -mass.VY
			} else if mass.Y > universeSize {
				mass.Y = universeSize
				mass.VY = -mass.VY
			}

			if err := g.writeMass(mass, t); err != nil {
				return err
			}
		}
	}

	// Check for proximity-based collisions
	if err := g.checkCollisions(t); err != nil {
		return err
	}

	// ~5% chance of new asteroid formation
	if g.rng.Float64() < 0.05 {
		if err := g.createAsteroid(t, ""); err != nil {
			return err
		}
	}

	return nil
}

// collisionPair represents two masses that are close enough to collide
type collisionPair struct {
	m1, m2   *Mass
	distance float64
}

// checkCollisions finds masses that are close enough to collide and handles one collision per tick
func (g *Generator) checkCollisions(t time.Time) error {
	// Find all pairs within collision distance
	var pairs []collisionPair

	for i, m1 := range g.masses {
		if !m1.Alive || m1.Kind == KindStar {
			continue
		}
		for j := i + 1; j < len(g.masses); j++ {
			m2 := g.masses[j]
			if !m2.Alive || m2.Kind == KindStar {
				continue
			}

			// Calculate distance between centers
			dx := m1.X - m2.X
			dy := m1.Y - m2.Y
			dist := math.Sqrt(dx*dx + dy*dy)

			// Collision when objects actually overlap (distance < sum of radii)
			threshold := m1.Radius + m2.Radius

			if dist < threshold {
				pairs = append(pairs, collisionPair{m1: m1, m2: m2, distance: dist})
			}
		}
	}

	if len(pairs) == 0 {
		return nil
	}

	// Handle the closest collision (most inevitable)
	closest := pairs[0]
	for _, p := range pairs[1:] {
		if p.distance < closest.distance {
			closest = p
		}
	}

	return g.handleCollision(closest.m1, closest.m2, t)
}

func (g *Generator) handleCollision(m1, m2 *Mass, t time.Time) error {
	// Determine larger and smaller
	smaller := m1
	larger := m2
	if m1.Radius > m2.Radius {
		smaller, larger = m2, m1
	}

	// Size ratio determines outcome
	sizeRatio := larger.Radius / smaller.Radius

	// Destroy smaller mass
	smaller.Alive = false
	if err := g.writeMass(smaller, t); err != nil {
		return err
	}

	if sizeRatio >= 3.0 {
		// Absorption: larger cleanly absorbs smaller, grows slightly
		larger.Radius += smaller.Radius * 0.25
		if err := g.writeMass(larger, t); err != nil {
			return err
		}

		desc := fmt.Sprintf("%s absorbed %s", larger.Name, smaller.Name)
		return g.writeEvent(EventAbsorption, desc, larger.ID, t)
	}

	// Violent collision: similar sizes, debris created
	desc := fmt.Sprintf("%s collided with %s", smaller.Name, larger.Name)
	if err := g.writeEvent(EventCollision, desc, smaller.ID, t); err != nil {
		return err
	}
	if err := g.writeEvent(EventDestruction, fmt.Sprintf("%s destroyed", smaller.Name), smaller.ID, t); err != nil {
		return err
	}

	// Create debris at collision site
	debris := &Mass{
		ID:     g.nextMassID(),
		Kind:   KindDebris,
		Name:   fmt.Sprintf("Debris-%03d", g.massID),
		X:      smaller.X,
		Y:      smaller.Y,
		Radius: 0.2,
		Alive:  true,
	}
	g.masses = append(g.masses, debris)
	return g.writeMass(debris, t)
}

func (g *Generator) nextMassID() string {
	g.massID++
	return fmt.Sprintf("mass-%03d", g.massID)
}

func (g *Generator) writeMass(m *Mass, t time.Time) error {
	e := &entity.Entity{
		ID:        m.ID,
		Type:      "mass",
		Timestamp: t.UnixNano(),
		Fields: map[string]entity.Value{
			"kind":   entity.NewString(m.Kind),
			"name":   entity.NewString(m.Name),
			"x":      entity.NewFloat(m.X),
			"y":      entity.NewFloat(m.Y),
			"radius": entity.NewFloat(m.Radius),
			"alive":  entity.NewBool(m.Alive),
		},
	}
	return g.store.Write(e)
}

func (g *Generator) writeEvent(eventType, description, massID string, t time.Time) error {
	g.eventID++
	e := &entity.Entity{
		ID:        fmt.Sprintf("event-%03d", g.eventID),
		Type:      "event",
		Timestamp: t.UnixNano(),
		Fields: map[string]entity.Value{
			"event_type":  entity.NewString(eventType),
			"description": entity.NewString(description),
			"mass_id":     entity.NewString(massID),
		},
	}
	return g.store.Write(e)
}
