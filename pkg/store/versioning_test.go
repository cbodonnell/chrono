package store_test

import (
	"testing"
	"time"

	"github.com/cbodonnell/chrono/pkg/entity"
	"github.com/cbodonnell/chrono/pkg/index"
	"github.com/cbodonnell/chrono/pkg/store"
)

func setupVersioningTestStore(t *testing.T) *store.EntityStore {
	kv := store.NewMemoryKVStore()
	idx := store.NewMemoryIndexStore()
	registry := index.NewRegistry()

	// Configure gamestate entity type
	registry.Register("gamestate", &index.EntityTypeConfig{
		Indexes: []index.FieldIndex{
			{Name: "score", Type: index.FieldTypeInt, Path: mustParsePath("score")},
		},
	})

	es, err := store.NewEntityStore(kv, idx, registry, store.NewMsgpackSerializer())
	if err != nil {
		t.Fatalf("failed to create entity store: %v", err)
	}

	return es
}

func TestVersionedWrite(t *testing.T) {
	s := setupVersioningTestStore(t)
	defer s.Close(t.Context())

	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()

	// Write multiple versions of the same entity
	for i := 0; i < 3; i++ {
		e := &entity.Entity{
			ID:        "game-1",
			Type:      "gamestate",
			Timestamp: baseTime + int64(i*1000),
			Fields: map[string]entity.Value{
				"score": entity.NewInt(int64((i + 1) * 100)),
			},
		}
		if err := s.Write(e); err != nil {
			t.Fatalf("Write version %d failed: %v", i, err)
		}
	}

	// Verify all versions were stored by checking history
	history, err := s.GetHistory("gamestate", "game-1", nil)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(history) != 3 {
		t.Errorf("Expected 3 versions, got %d", len(history))
	}
}

func TestGetLatestVersion(t *testing.T) {
	s := setupVersioningTestStore(t)
	defer s.Close(t.Context())

	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()

	// Write multiple versions
	for i := 0; i < 3; i++ {
		e := &entity.Entity{
			ID:        "game-1",
			Type:      "gamestate",
			Timestamp: baseTime + int64(i*1000),
			Fields: map[string]entity.Value{
				"score": entity.NewInt(int64((i + 1) * 100)),
			},
		}
		if err := s.Write(e); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Get should return the latest version
	latest, err := s.Get("gamestate", "game-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if latest.Fields["score"].I != 300 {
		t.Errorf("Expected score=300 (latest), got %d", latest.Fields["score"].I)
	}

	if latest.Timestamp != baseTime+2000 {
		t.Errorf("Expected timestamp=%d, got %d", baseTime+2000, latest.Timestamp)
	}
}

func TestGetVersion(t *testing.T) {
	s := setupVersioningTestStore(t)
	defer s.Close(t.Context())

	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()

	// Write multiple versions
	for i := 0; i < 3; i++ {
		e := &entity.Entity{
			ID:        "game-1",
			Type:      "gamestate",
			Timestamp: baseTime + int64(i*1000),
			Fields: map[string]entity.Value{
				"score": entity.NewInt(int64((i + 1) * 100)),
			},
		}
		if err := s.Write(e); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Get specific version (the middle one)
	version, err := s.GetVersion("gamestate", "game-1", baseTime+1000)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}

	if version.Fields["score"].I != 200 {
		t.Errorf("Expected score=200 for middle version, got %d", version.Fields["score"].I)
	}

	// Try to get non-existent version
	_, err = s.GetVersion("gamestate", "game-1", baseTime+5000)
	if err != store.ErrNotFound {
		t.Errorf("Expected ErrNotFound for non-existent version, got %v", err)
	}
}

func TestGetHistory(t *testing.T) {
	s := setupVersioningTestStore(t)
	defer s.Close(t.Context())

	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()

	// Write 5 versions
	for i := 0; i < 5; i++ {
		e := &entity.Entity{
			ID:        "game-1",
			Type:      "gamestate",
			Timestamp: baseTime + int64(i*1000),
			Fields: map[string]entity.Value{
				"score": entity.NewInt(int64((i + 1) * 100)),
			},
		}
		if err := s.Write(e); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Get all history (default: chronological order)
	history, err := s.GetHistory("gamestate", "game-1", nil)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(history) != 5 {
		t.Fatalf("Expected 5 versions, got %d", len(history))
	}

	// Verify chronological order
	for i, e := range history {
		expectedScore := int64((i + 1) * 100)
		if e.Fields["score"].I != expectedScore {
			t.Errorf("Version %d: expected score=%d, got %d", i, expectedScore, e.Fields["score"].I)
		}
	}

	// Get history in reverse order (newest first)
	historyReverse, err := s.GetHistory("gamestate", "game-1", &store.HistoryOptions{
		Reverse: true,
	})
	if err != nil {
		t.Fatalf("GetHistory reverse failed: %v", err)
	}

	if len(historyReverse) != 5 {
		t.Fatalf("Expected 5 versions in reverse, got %d", len(historyReverse))
	}

	// Verify reverse order
	if historyReverse[0].Fields["score"].I != 500 {
		t.Errorf("First in reverse should be score=500, got %d", historyReverse[0].Fields["score"].I)
	}
	if historyReverse[4].Fields["score"].I != 100 {
		t.Errorf("Last in reverse should be score=100, got %d", historyReverse[4].Fields["score"].I)
	}

	// Get history with limit
	historyLimited, err := s.GetHistory("gamestate", "game-1", &store.HistoryOptions{
		Limit: 2,
	})
	if err != nil {
		t.Fatalf("GetHistory limited failed: %v", err)
	}

	if len(historyLimited) != 2 {
		t.Errorf("Expected 2 versions with limit, got %d", len(historyLimited))
	}

	// Get history with time range
	historyRange, err := s.GetHistory("gamestate", "game-1", &store.HistoryOptions{
		TimeRange: &store.TimeRange{
			From: baseTime + 1000,
			To:   baseTime + 3000,
		},
	})
	if err != nil {
		t.Fatalf("GetHistory with range failed: %v", err)
	}

	if len(historyRange) != 3 {
		t.Errorf("Expected 3 versions in range, got %d", len(historyRange))
	}
}

func TestDeleteVersion(t *testing.T) {
	s := setupVersioningTestStore(t)
	defer s.Close(t.Context())

	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()

	// Write 3 versions
	for i := 0; i < 3; i++ {
		e := &entity.Entity{
			ID:        "game-1",
			Type:      "gamestate",
			Timestamp: baseTime + int64(i*1000),
			Fields: map[string]entity.Value{
				"score": entity.NewInt(int64((i + 1) * 100)),
			},
		}
		if err := s.Write(e); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Delete the middle version
	middleVersion, err := s.GetVersion("gamestate", "game-1", baseTime+1000)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}

	if err := s.Delete(middleVersion); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify only 2 versions remain
	history, err := s.GetHistory("gamestate", "game-1", nil)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(history) != 2 {
		t.Errorf("Expected 2 versions after delete, got %d", len(history))
	}

	// Verify deleted version is gone
	_, err = s.GetVersion("gamestate", "game-1", baseTime+1000)
	if err != store.ErrNotFound {
		t.Errorf("Expected ErrNotFound for deleted version, got %v", err)
	}

	// Get() should still return the latest version
	latest, err := s.Get("gamestate", "game-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if latest.Fields["score"].I != 300 {
		t.Errorf("Expected latest score=300, got %d", latest.Fields["score"].I)
	}
}

func TestDeleteEntity(t *testing.T) {
	s := setupVersioningTestStore(t)
	defer s.Close(t.Context())

	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()

	// Write 3 versions
	for i := 0; i < 3; i++ {
		e := &entity.Entity{
			ID:        "game-1",
			Type:      "gamestate",
			Timestamp: baseTime + int64(i*1000),
			Fields: map[string]entity.Value{
				"score": entity.NewInt(int64((i + 1) * 100)),
			},
		}
		if err := s.Write(e); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Delete all versions
	if err := s.DeleteEntity("gamestate", "game-1"); err != nil {
		t.Fatalf("DeleteEntity failed: %v", err)
	}

	// Verify all versions are gone
	_, err := s.Get("gamestate", "game-1")
	if err != store.ErrNotFound {
		t.Errorf("Expected ErrNotFound after DeleteEntity, got %v", err)
	}

	history, err := s.GetHistory("gamestate", "game-1", nil)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}

	if len(history) != 0 {
		t.Errorf("Expected 0 versions after DeleteEntity, got %d", len(history))
	}
}

func TestQueryWithVersioning(t *testing.T) {
	s := setupVersioningTestStore(t)
	defer s.Close(t.Context())

	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano()

	// Write multiple versions of two entities
	for i := 0; i < 3; i++ {
		// game-1 with increasing scores
		e1 := &entity.Entity{
			ID:        "game-1",
			Type:      "gamestate",
			Timestamp: baseTime + int64(i*1000),
			Fields: map[string]entity.Value{
				"score": entity.NewInt(int64((i + 1) * 100)),
			},
		}
		if err := s.Write(e1); err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		// game-2 with different scores
		e2 := &entity.Entity{
			ID:        "game-2",
			Type:      "gamestate",
			Timestamp: baseTime + int64(i*1000) + 500,
			Fields: map[string]entity.Value{
				"score": entity.NewInt(int64((i + 1) * 50)),
			},
		}
		if err := s.Write(e2); err != nil {
			t.Fatalf("Write failed: %v", err)
		}
	}

	// Query without IncludeHistory (default) - should return only latest version per entity
	results, err := s.Query(&store.Query{
		EntityType: "gamestate",
	})
	if err != nil {
		t.Fatalf("Query failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 entities (latest versions only), got %d", len(results))
	}

	// Verify we got the latest versions
	for _, e := range results {
		if e.ID == "game-1" && e.Fields["score"].I != 300 {
			t.Errorf("game-1 should have latest score=300, got %d", e.Fields["score"].I)
		}
		if e.ID == "game-2" && e.Fields["score"].I != 150 {
			t.Errorf("game-2 should have latest score=150, got %d", e.Fields["score"].I)
		}
	}

	// Query with IncludeHistory - should return all versions
	resultsWithHistory, err := s.Query(&store.Query{
		EntityType:     "gamestate",
		IncludeHistory: true,
	})
	if err != nil {
		t.Fatalf("Query with history failed: %v", err)
	}

	if len(resultsWithHistory) != 6 {
		t.Errorf("Expected 6 versions (all history), got %d", len(resultsWithHistory))
	}

	// Query by field with deduplication
	resultsFiltered, err := s.Query(&store.Query{
		EntityType: "gamestate",
		Filters: []store.FieldFilter{
			{Field: "score", Op: store.OpGte, Value: entity.NewInt(100)},
		},
	})
	if err != nil {
		t.Fatalf("Query filtered failed: %v", err)
	}

	// Should return only 2 entities (latest versions) that match the filter
	if len(resultsFiltered) != 2 {
		t.Errorf("Expected 2 entities with score >= 100, got %d", len(resultsFiltered))
	}
}

func TestRetentionWithVersioning(t *testing.T) {
	kv := store.NewMemoryKVStore()
	idx := store.NewMemoryIndexStore()
	registry := index.NewRegistry()

	// Configure with TTL
	registry.Register("gamestate", &index.EntityTypeConfig{
		Indexes: []index.FieldIndex{
			{Name: "score", Type: index.FieldTypeInt, Path: mustParsePath("score")},
		},
		TTL: time.Hour, // 1 hour TTL
	})

	s, err := store.NewEntityStore(kv, idx, registry, store.NewMsgpackSerializer())
	if err != nil {
		t.Fatalf("failed to create entity store: %v", err)
	}
	defer s.Close(t.Context())

	now := time.Now().UnixNano()
	oldTime := now - int64(2*time.Hour) // 2 hours ago (expired)

	// Write old and new versions
	oldVersion := &entity.Entity{
		ID:        "game-1",
		Type:      "gamestate",
		Timestamp: oldTime,
		Fields: map[string]entity.Value{
			"score": entity.NewInt(100),
		},
	}
	if err := s.Write(oldVersion); err != nil {
		t.Fatalf("Write old version failed: %v", err)
	}

	newVersion := &entity.Entity{
		ID:        "game-1",
		Type:      "gamestate",
		Timestamp: now,
		Fields: map[string]entity.Value{
			"score": entity.NewInt(200),
		},
	}
	if err := s.Write(newVersion); err != nil {
		t.Fatalf("Write new version failed: %v", err)
	}

	// Verify both versions exist
	history, err := s.GetHistory("gamestate", "game-1", nil)
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("Expected 2 versions before retention, got %d", len(history))
	}

	// Run retention to delete expired versions
	cutoff := now - int64(time.Hour) // 1 hour ago
	deleted, err := s.DeleteExpiredBatch("gamestate", cutoff, 100)
	if err != nil {
		t.Fatalf("DeleteExpiredBatch failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("Expected 1 deleted, got %d", deleted)
	}

	// Verify only new version remains
	history, err = s.GetHistory("gamestate", "game-1", nil)
	if err != nil {
		t.Fatalf("GetHistory after retention failed: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("Expected 1 version after retention, got %d", len(history))
	}

	if history[0].Fields["score"].I != 200 {
		t.Errorf("Expected score=200 for remaining version, got %d", history[0].Fields["score"].I)
	}
}
