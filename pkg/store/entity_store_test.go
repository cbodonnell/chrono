package store_test

import (
	"testing"
	"time"

	"github.com/cbodonnell/chrono/pkg/entity"
	"github.com/cbodonnell/chrono/pkg/index"
	"github.com/cbodonnell/chrono/pkg/store"
)

func setupTestStore() *store.EntityStore {
	kv := store.NewMemoryKVStore()
	idx := store.NewMemoryIndexStore()
	registry := index.NewRegistry()

	// Configure sensor entity type
	registry.Register("sensor", &index.EntityTypeConfig{
		Indexes: []index.FieldIndex{
			{Name: "temp", Type: index.FieldTypeFloat, Path: mustParsePath("temp")},
			{Name: "active", Type: index.FieldTypeBool, Path: mustParsePath("active")},
			{Name: "tags", Type: index.FieldTypeStringArray, Path: mustParsePath("tags")},
		},
	})

	return store.NewEntityStore(kv, idx, registry, store.NewMsgpackSerializer())
}

func mustParsePath(s string) entity.Path {
	path, err := entity.ParsePath(s)
	if err != nil {
		panic(err)
	}
	return path
}

func setupNestedTestStore() *store.EntityStore {
	kv := store.NewMemoryKVStore()
	idx := store.NewMemoryIndexStore()
	registry := index.NewRegistry()

	// Configure device entity type with nested indexes
	registry.Register("device", &index.EntityTypeConfig{
		Indexes: []index.FieldIndex{
			{Name: "status", Type: index.FieldTypeString, Path: mustParsePath("status")},
			{Name: "metadata.location", Type: index.FieldTypeString, Path: mustParsePath("metadata.location")},
			{Name: "readings[0].value", Type: index.FieldTypeFloat, Path: mustParsePath("readings[0].value")},
		},
	})

	return store.NewEntityStore(kv, idx, registry, store.NewMsgpackSerializer())
}

func TestNestedFieldIndex(t *testing.T) {
	s := setupNestedTestStore()
	defer s.Close()

	now := time.Now().UnixNano()

	// Create device with nested fields
	device := &entity.Entity{
		ID:        "device-001",
		Type:      "device",
		Timestamp: now,
		Fields: map[string]entity.Value{
			"status": entity.NewString("active"),
			"metadata": entity.NewObject(map[string]entity.Value{
				"location": entity.NewString("building-a"),
				"owner":    entity.NewString("team-x"),
			}),
			"readings": entity.NewArray([]entity.Value{
				entity.NewObject(map[string]entity.Value{
					"value": entity.NewFloat(72.5),
					"unit":  entity.NewString("F"),
				}),
				entity.NewObject(map[string]entity.Value{
					"value": entity.NewFloat(23.1),
					"unit":  entity.NewString("C"),
				}),
			}),
		},
	}

	if err := s.Write(device); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Query by nested object field
	results, err := s.Query(&store.Query{
		EntityType: "device",
		Filters: []store.FieldFilter{
			{Field: "metadata.location", Op: store.OpEq, Value: entity.NewString("building-a")},
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result for metadata.location, got %d", len(results))
	}
	if len(results) > 0 && results[0].ID != "device-001" {
		t.Errorf("Expected device-001, got %s", results[0].ID)
	}
}

func TestNestedArrayFieldIndex(t *testing.T) {
	s := setupNestedTestStore()
	defer s.Close()

	now := time.Now().UnixNano()

	// Create multiple devices with different readings
	devices := []*entity.Entity{
		{
			ID:        "device-001",
			Type:      "device",
			Timestamp: now,
			Fields: map[string]entity.Value{
				"status": entity.NewString("active"),
				"readings": entity.NewArray([]entity.Value{
					entity.NewObject(map[string]entity.Value{
						"value": entity.NewFloat(72.5),
					}),
				}),
			},
		},
		{
			ID:        "device-002",
			Type:      "device",
			Timestamp: now + 1000,
			Fields: map[string]entity.Value{
				"status": entity.NewString("active"),
				"readings": entity.NewArray([]entity.Value{
					entity.NewObject(map[string]entity.Value{
						"value": entity.NewFloat(68.0),
					}),
				}),
			},
		},
		{
			ID:        "device-003",
			Type:      "device",
			Timestamp: now + 2000,
			Fields: map[string]entity.Value{
				"status": entity.NewString("inactive"),
				"readings": entity.NewArray([]entity.Value{
					entity.NewObject(map[string]entity.Value{
						"value": entity.NewFloat(72.5),
					}),
				}),
			},
		},
	}

	for _, d := range devices {
		if err := s.Write(d); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Query by array element's nested field
	results, err := s.Query(&store.Query{
		EntityType: "device",
		Filters: []store.FieldFilter{
			{Field: "readings[0].value", Op: store.OpEq, Value: entity.NewFloat(72.5)},
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results for readings[0].value=72.5, got %d", len(results))
	}
}

func TestDeleteNestedFieldIndex(t *testing.T) {
	s := setupNestedTestStore()
	defer s.Close()

	device := &entity.Entity{
		ID:        "device-001",
		Type:      "device",
		Timestamp: time.Now().UnixNano(),
		Fields: map[string]entity.Value{
			"status": entity.NewString("active"),
			"metadata": entity.NewObject(map[string]entity.Value{
				"location": entity.NewString("building-a"),
			}),
		},
	}

	if err := s.Write(device); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify entity can be queried
	results, err := s.Query(&store.Query{
		EntityType: "device",
		Filters: []store.FieldFilter{
			{Field: "metadata.location", Op: store.OpEq, Value: entity.NewString("building-a")},
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Expected 1 result before delete, got %d", len(results))
	}

	// Delete the entity
	if err := s.Delete(device); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify entity is gone
	results, err = s.Query(&store.Query{
		EntityType: "device",
		Filters: []store.FieldFilter{
			{Field: "metadata.location", Op: store.OpEq, Value: entity.NewString("building-a")},
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results after delete, got %d", len(results))
	}
}

func TestWriteAndGet(t *testing.T) {
	s := setupTestStore()
	defer s.Close()

	e := &entity.Entity{
		ID:        "sensor-001",
		Type:      "sensor",
		Timestamp: time.Now().UnixNano(),
		Fields: map[string]entity.Value{
			"temp":   entity.NewFloat(72.5),
			"active": entity.NewBool(true),
			"tags":   entity.NewStringArray([]string{"A", "B"}),
		},
	}

	if err := s.Write(e); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, err := s.Get("sensor", "sensor-001")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.ID != e.ID {
		t.Errorf("ID mismatch: got %s, want %s", got.ID, e.ID)
	}
	if got.Type != e.Type {
		t.Errorf("Type mismatch: got %s, want %s", got.Type, e.Type)
	}
	if got.Fields["temp"].F != 72.5 {
		t.Errorf("temp mismatch: got %v, want 72.5", got.Fields["temp"].F)
	}
}

func TestQueryByField(t *testing.T) {
	s := setupTestStore()
	defer s.Close()

	now := time.Now().UnixNano()

	// Write multiple sensors
	entities := []*entity.Entity{
		{
			ID:        "sensor-001",
			Type:      "sensor",
			Timestamp: now,
			Fields: map[string]entity.Value{
				"temp":   entity.NewFloat(72.5),
				"active": entity.NewBool(true),
			},
		},
		{
			ID:        "sensor-002",
			Type:      "sensor",
			Timestamp: now + 1000,
			Fields: map[string]entity.Value{
				"temp":   entity.NewFloat(68.0),
				"active": entity.NewBool(false),
			},
		},
		{
			ID:        "sensor-003",
			Type:      "sensor",
			Timestamp: now + 2000,
			Fields: map[string]entity.Value{
				"temp":   entity.NewFloat(72.5),
				"active": entity.NewBool(true),
			},
		},
	}

	for _, e := range entities {
		if err := s.Write(e); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Query by active = true
	results, err := s.Query(&store.Query{
		EntityType: "sensor",
		Filters: []store.FieldFilter{
			{Field: "active", Op: store.OpEq, Value: entity.NewBool(true)},
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}
}

func TestQueryByArrayContains(t *testing.T) {
	s := setupTestStore()
	defer s.Close()

	now := time.Now().UnixNano()

	entities := []*entity.Entity{
		{
			ID:        "sensor-001",
			Type:      "sensor",
			Timestamp: now,
			Fields: map[string]entity.Value{
				"tags": entity.NewStringArray([]string{"A", "B"}),
			},
		},
		{
			ID:        "sensor-002",
			Type:      "sensor",
			Timestamp: now + 1000,
			Fields: map[string]entity.Value{
				"tags": entity.NewStringArray([]string{"B", "C"}),
			},
		},
		{
			ID:        "sensor-003",
			Type:      "sensor",
			Timestamp: now + 2000,
			Fields: map[string]entity.Value{
				"tags": entity.NewStringArray([]string{"C", "D"}),
			},
		},
	}

	for _, e := range entities {
		if err := s.Write(e); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Query by tags contains "B"
	results, err := s.Query(&store.Query{
		EntityType: "sensor",
		Filters: []store.FieldFilter{
			{Field: "tags", Op: store.OpContains, Value: entity.NewString("B")},
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results (sensors with tag B), got %d", len(results))
	}
}

func TestQueryTimeRange(t *testing.T) {
	s := setupTestStore()
	defer s.Close()

	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()

	entities := []*entity.Entity{
		{
			ID:        "sensor-001",
			Type:      "sensor",
			Timestamp: baseTime,
			Fields: map[string]entity.Value{
				"temp": entity.NewFloat(70.0),
			},
		},
		{
			ID:        "sensor-002",
			Type:      "sensor",
			Timestamp: baseTime + int64(time.Hour),
			Fields: map[string]entity.Value{
				"temp": entity.NewFloat(72.0),
			},
		},
		{
			ID:        "sensor-003",
			Type:      "sensor",
			Timestamp: baseTime + int64(2*time.Hour),
			Fields: map[string]entity.Value{
				"temp": entity.NewFloat(74.0),
			},
		},
	}

	for _, e := range entities {
		if err := s.Write(e); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Query with time range (first 90 minutes)
	results, err := s.Query(&store.Query{
		EntityType: "sensor",
		TimeRange: &store.TimeRange{
			From: baseTime,
			To:   baseTime + int64(90*time.Minute),
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results in time range, got %d", len(results))
	}
}

func TestCompoundQuery(t *testing.T) {
	s := setupTestStore()
	defer s.Close()

	now := time.Now().UnixNano()

	entities := []*entity.Entity{
		{
			ID:        "sensor-001",
			Type:      "sensor",
			Timestamp: now,
			Fields: map[string]entity.Value{
				"temp":   entity.NewFloat(72.5),
				"active": entity.NewBool(true),
				"tags":   entity.NewStringArray([]string{"production"}),
			},
		},
		{
			ID:        "sensor-002",
			Type:      "sensor",
			Timestamp: now + 1000,
			Fields: map[string]entity.Value{
				"temp":   entity.NewFloat(72.5),
				"active": entity.NewBool(false),
				"tags":   entity.NewStringArray([]string{"production"}),
			},
		},
		{
			ID:        "sensor-003",
			Type:      "sensor",
			Timestamp: now + 2000,
			Fields: map[string]entity.Value{
				"temp":   entity.NewFloat(68.0),
				"active": entity.NewBool(true),
				"tags":   entity.NewStringArray([]string{"staging"}),
			},
		},
	}

	for _, e := range entities {
		if err := s.Write(e); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Query: active = true AND tags contains "production"
	results, err := s.Query(&store.Query{
		EntityType: "sensor",
		Filters: []store.FieldFilter{
			{Field: "active", Op: store.OpEq, Value: entity.NewBool(true)},
			{Field: "tags", Op: store.OpContains, Value: entity.NewString("production")},
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result (active + production), got %d", len(results))
	}

	if len(results) > 0 && results[0].ID != "sensor-001" {
		t.Errorf("Expected sensor-001, got %s", results[0].ID)
	}
}

func TestWriteUpdatesIndexes(t *testing.T) {
	s := setupTestStore()
	defer s.Close()

	now := time.Now().UnixNano()

	// Write initial entity with active=true
	e := &entity.Entity{
		ID:        "sensor-001",
		Type:      "sensor",
		Timestamp: now,
		Fields: map[string]entity.Value{
			"temp":   entity.NewFloat(72.5),
			"active": entity.NewBool(true),
		},
	}

	if err := s.Write(e); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Verify it can be queried by active=true
	results, err := s.Query(&store.Query{
		EntityType: "sensor",
		Filters: []store.FieldFilter{
			{Field: "active", Op: store.OpEq, Value: entity.NewBool(true)},
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result for active=true, got %d", len(results))
	}

	// Update the entity with active=false (and new timestamp)
	e.Timestamp = now + 1000
	e.Fields["active"] = entity.NewBool(false)

	if err := s.Write(e); err != nil {
		t.Fatalf("Write update failed: %v", err)
	}

	// Old index should be gone - query by active=true should return 0
	results, err = s.Query(&store.Query{
		EntityType: "sensor",
		Filters: []store.FieldFilter{
			{Field: "active", Op: store.OpEq, Value: entity.NewBool(true)},
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 results for active=true after update, got %d", len(results))
	}

	// New index should exist - query by active=false should return 1
	results, err = s.Query(&store.Query{
		EntityType: "sensor",
		Filters: []store.FieldFilter{
			{Field: "active", Op: store.OpEq, Value: entity.NewBool(false)},
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 result for active=false after update, got %d", len(results))
	}

	// Verify Get returns the updated entity
	got, err := s.Get("sensor", "sensor-001")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Fields["active"].B != false {
		t.Errorf("Expected active=false, got %v", got.Fields["active"].B)
	}
	if got.Timestamp != now+1000 {
		t.Errorf("Expected updated timestamp, got %d", got.Timestamp)
	}
}

func TestDelete(t *testing.T) {
	s := setupTestStore()
	defer s.Close()

	e := &entity.Entity{
		ID:        "sensor-001",
		Type:      "sensor",
		Timestamp: time.Now().UnixNano(),
		Fields: map[string]entity.Value{
			"temp":   entity.NewFloat(72.5),
			"active": entity.NewBool(true),
		},
	}

	if err := s.Write(e); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := s.Delete(e); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := s.Get("sensor", "sensor-001")
	if err != store.ErrNotFound {
		t.Errorf("Expected ErrNotFound after delete, got %v", err)
	}

	// Query should return no results
	results, err := s.Query(&store.Query{
		EntityType: "sensor",
		Filters: []store.FieldFilter{
			{Field: "active", Op: store.OpEq, Value: entity.NewBool(true)},
		},
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("Expected 0 results after delete, got %d", len(results))
	}
}
