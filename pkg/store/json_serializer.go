package store

import (
	"encoding/json"

	"github.com/cbodonnell/chrono/pkg/entity"
)

// JSONSerializer implements Serializer using JSON encoding.
type JSONSerializer struct{}

// NewJSONSerializer creates a new JSON serializer.
func NewJSONSerializer() *JSONSerializer {
	return &JSONSerializer{}
}

// Marshal encodes an entity to JSON.
func (s *JSONSerializer) Marshal(e *entity.Entity) ([]byte, error) {
	return json.Marshal(e)
}

// Unmarshal decodes JSON into an entity.
func (s *JSONSerializer) Unmarshal(data []byte, e *entity.Entity) error {
	return json.Unmarshal(data, e)
}
