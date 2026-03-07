package store

import "github.com/cbodonnell/chrono/pkg/entity"

// Serializer handles encoding and decoding of entities.
type Serializer interface {
	Marshal(e *entity.Entity) ([]byte, error)
	Unmarshal(data []byte, e *entity.Entity) error
}
