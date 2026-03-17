package generate

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/cbodonnell/chrono/examples/space/internal/physics"
	"github.com/cbodonnell/chrono/pkg/entity"
	"github.com/cbodonnell/chrono/pkg/store"
)

const (
	// Universe parameters
	universeSize            = 100.0 // 0-100 coordinate space
	numSystems              = 2
	numPlanetsPerSystem     = 2
	numAsteroids            = 7
	asteroidFormationChance = 0.05 // 5% chance of new asteroid each tick
	TickCount               = 2400 // ticks (1 per minute)
	TickDuration            = time.Minute
)

// Physics parameters
const (
	G                = 0.0001 // Gravitational constant (tuned for stable orbits)
	SofteningEpsilon = 0.5    // Prevents singularities
	DT               = 1.0    // Time step (1 unit per tick)
)

// Mass ranges by type
const (
	StarMassMin     = 500.0
	StarMassMax     = 2000.0
	GiantMassMin    = 50.0
	GiantMassMax    = 200.0
	PlanetMassMin   = 5.0
	PlanetMassMax   = 45.0
	AsteroidMassMin = 0.5
	AsteroidMassMax = 3.0
)

// Planet composition chances
const (
	GasGiantChance = 0.3 // 30% of planets are gas giants
)

// Event types
const (
	EventFormation   = "formation"
	EventCollision   = "collision"
	EventDestruction = "destruction"
	EventAbsorption  = "absorption"
)

// Generator handles universe generation
type Generator struct {
	store   *store.EntityStore
	rng     *rand.Rand
	sim     *physics.Simulation
	massID  int
	eventID int
}

// New creates a new generator
func New(es *store.EntityStore, seed int64) *Generator {
	config := physics.Config{
		G:                G,
		SofteningEpsilon: SofteningEpsilon,
		UniverseSize:     universeSize,
	}

	return &Generator{
		store: es,
		rng:   rand.New(rand.NewSource(seed)),
		sim:   physics.NewSimulation(config),
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
	fmt.Printf("Created %d initial bodies\n", len(g.sim.Bodies))

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
	// Create star systems
	for i := range numSystems {
		if err := g.createStarSystem(t, i); err != nil {
			return err
		}
	}

	// Scatter asteroids throughout
	for range numAsteroids {
		if err := g.createAsteroid(t, ""); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) createStarSystem(t time.Time, systemIndex int) error {
	// Position star
	starX := universeSize*0.15 + g.rng.Float64()*universeSize*0.7
	starY := universeSize*0.15 + g.rng.Float64()*universeSize*0.7

	// Create central star (gas composition, massive enough for fusion)
	starMass := StarMassMin + g.rng.Float64()*(StarMassMax-StarMassMin)
	star := &physics.Body{
		ID:          g.nextMassID(),
		Name:        fmt.Sprintf("Sol-%d", systemIndex+1),
		Composition: physics.CompGas,
		Mass:        starMass,
		X:           starX,
		Y:           starY,
		Radius:      3.0 + g.rng.Float64()*2.0,
		Alive:       true,
	}
	g.sim.AddBody(star)
	if err := g.writeBody(star, t); err != nil {
		return err
	}
	if err := g.writeEvent(EventFormation, fmt.Sprintf("Star %s ignited", star.Name), star.ID, t); err != nil {
		return err
	}

	// Create planets orbiting the star
	for i := range numPlanetsPerSystem {
		orbitRadius := 8.0 + float64(i)*6.0 + g.rng.Float64()*3.0
		orbitAngle := g.rng.Float64() * 2 * math.Pi

		// Calculate orbital velocity: v = sqrt(G*M/r)
		orbitalSpeed := math.Sqrt(G * starMass / orbitRadius)

		// Velocity perpendicular to radius (counterclockwise)
		vx := -orbitalSpeed * math.Sin(orbitAngle)
		vy := orbitalSpeed * math.Cos(orbitAngle)

		// Determine if gas giant or rocky planet
		var composition string
		var planetMass float64
		var radius float64

		if g.rng.Float64() < GasGiantChance {
			// Gas giant
			composition = physics.CompGas
			planetMass = GiantMassMin + g.rng.Float64()*(GiantMassMax-GiantMassMin)
			radius = 2.0 + g.rng.Float64()*1.0
		} else {
			// Rocky planet
			composition = physics.CompRite
			planetMass = PlanetMassMin + g.rng.Float64()*(PlanetMassMax-PlanetMassMin)
			radius = 1.0 + g.rng.Float64()*1.5
		}

		planet := &physics.Body{
			ID:          g.nextMassID(),
			Name:        fmt.Sprintf("%s-%c", star.Name, 'a'+rune(i)),
			Composition: composition,
			Mass:        planetMass,
			X:           starX + orbitRadius*math.Cos(orbitAngle),
			Y:           starY + orbitRadius*math.Sin(orbitAngle),
			VX:          vx,
			VY:          vy,
			Radius:      radius,
			Alive:       true,
		}
		g.sim.AddBody(planet)
		if err := g.writeBody(planet, t); err != nil {
			return err
		}

		kindName := "Planet"
		if planet.Kind() == physics.KindGiant {
			kindName = "Gas giant"
		}
		if err := g.writeEvent(EventFormation, fmt.Sprintf("%s %s formed", kindName, planet.Name), planet.ID, t); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) createAsteroid(t time.Time, reason string) error {
	// Random velocity direction and speed
	angle := g.rng.Float64() * 2 * math.Pi
	speed := 0.5 + g.rng.Float64()*1.0

	asteroidMass := AsteroidMassMin + g.rng.Float64()*(AsteroidMassMax-AsteroidMassMin)
	asteroid := &physics.Body{
		ID:          g.nextMassID(),
		Name:        fmt.Sprintf("Ast-%03d", g.massID),
		Composition: physics.CompRite, // Asteroids are rocky
		Mass:        asteroidMass,
		X:           g.rng.Float64() * universeSize,
		Y:           g.rng.Float64() * universeSize,
		VX:          speed * math.Cos(angle),
		VY:          speed * math.Sin(angle),
		Radius:      0.5 + g.rng.Float64()*0.5,
		Alive:       true,
	}
	g.sim.AddBody(asteroid)
	if err := g.writeBody(asteroid, t); err != nil {
		return err
	}

	desc := fmt.Sprintf("Asteroid %s detected", asteroid.Name)
	if reason != "" {
		desc = reason
	}
	return g.writeEvent(EventFormation, desc, asteroid.ID, t)
}

func (g *Generator) evolveTick(_ int, t time.Time) error {
	// Run physics simulation step
	events := g.sim.Step(DT)

	// Write updated positions for all alive bodies
	for _, body := range g.sim.Bodies {
		if !body.Alive {
			continue
		}
		if err := g.writeBody(body, t); err != nil {
			return err
		}
	}

	// Process collision events
	for _, event := range events {
		if err := g.processCollisionEvent(event, t); err != nil {
			return err
		}
	}

	// ~5% chance of new asteroid formation
	if g.rng.Float64() < asteroidFormationChance {
		if err := g.createAsteroid(t, ""); err != nil {
			return err
		}
	}

	return nil
}

func (g *Generator) processCollisionEvent(event physics.CollisionEvent, t time.Time) error {
	b1 := g.sim.GetBody(event.Body1ID)
	b2 := g.sim.GetBody(event.Body2ID)

	if b1 == nil || b2 == nil {
		return nil // Bodies already removed
	}

	switch event.Type {
	case physics.CollisionAbsorption:
		// Find the destroyed body (the one that's not the survivor)
		var survivor, destroyed *physics.Body
		if event.SurvivorID == b1.ID {
			survivor, destroyed = b1, b2
		} else {
			survivor, destroyed = b2, b1
		}

		// Write the destroyed body's final state
		if err := g.writeBody(destroyed, t); err != nil {
			return err
		}

		// Write the survivor's updated state
		if err := g.writeBody(survivor, t); err != nil {
			return err
		}

		desc := fmt.Sprintf("%s absorbed %s", survivor.Name, destroyed.Name)
		return g.writeEvent(EventAbsorption, desc, survivor.ID, t)

	case physics.CollisionDestruction:
		// Write both destroyed bodies
		if err := g.writeBody(b1, t); err != nil {
			return err
		}
		if err := g.writeBody(b2, t); err != nil {
			return err
		}

		// Write collision event
		desc := fmt.Sprintf("%s collided with %s", b1.Name, b2.Name)
		if err := g.writeEvent(EventCollision, desc, b1.ID, t); err != nil {
			return err
		}

		// Write destruction events
		if err := g.writeEvent(EventDestruction, fmt.Sprintf("%s destroyed", b1.Name), b1.ID, t); err != nil {
			return err
		}
		if err := g.writeEvent(EventDestruction, fmt.Sprintf("%s destroyed", b2.Name), b2.ID, t); err != nil {
			return err
		}

		// Assign IDs and names to collision products (name based on derived kind)
		for _, piece := range event.Debris {
			piece.ID = g.nextMassID()
			// Name based on what the piece actually is
			switch piece.Kind() {
			case physics.KindStar:
				piece.Name = fmt.Sprintf("Nova-%03d", g.massID)
			case physics.KindGiant:
				piece.Name = fmt.Sprintf("Giant-%03d", g.massID)
			case physics.KindCloud:
				piece.Name = fmt.Sprintf("Cloud-%03d", g.massID)
			case physics.KindPlanet:
				piece.Name = fmt.Sprintf("Planemo-%03d", g.massID) // Rogue planet
			case physics.KindAsteroid:
				piece.Name = fmt.Sprintf("Ast-%03d", g.massID)
			default:
				piece.Name = fmt.Sprintf("Debris-%03d", g.massID)
			}
			if err := g.writeBody(piece, t); err != nil {
				return err
			}
		}
	}

	return nil
}

func (g *Generator) nextMassID() string {
	g.massID++
	return fmt.Sprintf("mass-%03d", g.massID)
}

func (g *Generator) writeBody(b *physics.Body, t time.Time) error {
	e := &entity.Entity{
		ID:        b.ID,
		Type:      "mass",
		Timestamp: t.UnixNano(),
		Fields: map[string]entity.Value{
			"kind":        entity.NewString(b.Kind()), // Derived from mass + composition
			"composition": entity.NewString(b.Composition),
			"name":        entity.NewString(b.Name),
			"mass":        entity.NewFloat(b.Mass),
			"x":           entity.NewFloat(b.X),
			"y":           entity.NewFloat(b.Y),
			"vx":          entity.NewFloat(b.VX),
			"vy":          entity.NewFloat(b.VY),
			"radius":      entity.NewFloat(b.Radius),
			"alive":       entity.NewBool(b.Alive),
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
