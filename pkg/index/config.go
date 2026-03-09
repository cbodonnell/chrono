package index

import (
	"sync"
	"time"

	"github.com/cbodonnell/chrono/pkg/entity"
)

// FieldType represents the type of an indexed field.
type FieldType uint8

const (
	FieldTypeInt FieldType = iota
	FieldTypeFloat
	FieldTypeBool
	FieldTypeString
	FieldTypeIntArray
	FieldTypeFloatArray
	FieldTypeBoolArray
	FieldTypeStringArray
)

// IsArray returns true if this field type is an array type.
func (ft FieldType) IsArray() bool {
	return ft >= FieldTypeIntArray
}

// ElementKind returns the ValueKind for the elements of this field type.
func (ft FieldType) ElementKind() entity.ValueKind {
	switch ft {
	case FieldTypeInt, FieldTypeIntArray:
		return entity.KindInt
	case FieldTypeFloat, FieldTypeFloatArray:
		return entity.KindFloat
	case FieldTypeBool, FieldTypeBoolArray:
		return entity.KindBool
	case FieldTypeString, FieldTypeStringArray:
		return entity.KindString
	default:
		return entity.KindString
	}
}

// FieldIndex defines an index on a single field.
type FieldIndex struct {
	Name string // Original path string (e.g., "user.address.city")
	Type FieldType
	Path entity.Path // Parsed path segments (computed once at registration)
}

// EntityTypeConfig holds the index configuration for an entity type.
type EntityTypeConfig struct {
	Indexes []FieldIndex
	TTL     time.Duration // 0 means no TTL
}

// Registry holds index configurations for all entity types.
type Registry struct {
	mu      sync.RWMutex
	configs map[string]*EntityTypeConfig
}

// NewRegistry creates a new empty index registry.
func NewRegistry() *Registry {
	return &Registry{
		configs: make(map[string]*EntityTypeConfig),
	}
}

// Register adds or updates the configuration for an entity type.
func (r *Registry) Register(entityType string, config *EntityTypeConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs[entityType] = config
}

// Get retrieves the configuration for an entity type.
// Returns nil if no configuration exists.
func (r *Registry) Get(entityType string) *EntityTypeConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.configs[entityType]
}

// Has returns true if a configuration exists for the entity type.
func (r *Registry) Has(entityType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.configs[entityType]
	return ok
}

// EntityTypes returns a list of all registered entity types.
func (r *Registry) EntityTypes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	types := make([]string, 0, len(r.configs))
	for t := range r.configs {
		types = append(types, t)
	}
	return types
}

// HasRetention returns true if any entity has retention configured
func (r *Registry) HasRetention() bool {
	for _, entityType := range r.EntityTypes() {
		cfg := r.Get(entityType)
		if cfg != nil && cfg.TTL > 0 {
			return true
		}
	}
	return false
}
