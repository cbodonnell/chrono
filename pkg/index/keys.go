package index

import (
	"bytes"

	"github.com/cbodonnell/chrono/pkg/encoding"
	"github.com/cbodonnell/chrono/pkg/entity"
)

// Separator is used between key components.
const Separator = '/'

// AllFieldName is the synthetic field name for time-series queries across all entities.
const AllFieldName = "_all"

// ByIDFieldName is the synthetic field name for the _by_id index (entity versions).
const ByIDFieldName = "_by_id"

// KeyBuilder builds index keys following the schema:
// {entity_type}/{field_name}/{field_value_bytes}/{timestamp_unix_ns}/{entity_id}
type KeyBuilder struct {
	buf bytes.Buffer
}

// NewKeyBuilder creates a new KeyBuilder.
func NewKeyBuilder() *KeyBuilder {
	return &KeyBuilder{}
}

// Reset clears the buffer for reuse.
func (kb *KeyBuilder) Reset() {
	kb.buf.Reset()
}

// BuildKey constructs an index key for a field value.
func (kb *KeyBuilder) BuildKey(entityType, fieldName string, value entity.Value, timestamp int64, entityID string) []byte {
	kb.Reset()
	kb.buf.WriteString(entityType)
	kb.buf.WriteByte(Separator)
	kb.buf.WriteString(fieldName)
	kb.buf.WriteByte(Separator)
	kb.buf.Write(EncodeValue(value))
	kb.buf.WriteByte(Separator)
	kb.buf.Write(encoding.EncodeTimestamp(timestamp))
	kb.buf.WriteByte(Separator)
	kb.buf.WriteString(entityID)
	return kb.copyBytes()
}

// BuildAllKey constructs the synthetic _all index key for time-series queries.
func (kb *KeyBuilder) BuildAllKey(entityType string, timestamp int64, entityID string) []byte {
	kb.Reset()
	kb.buf.WriteString(entityType)
	kb.buf.WriteByte(Separator)
	kb.buf.WriteString(AllFieldName)
	kb.buf.WriteByte(Separator)
	kb.buf.Write(encoding.EncodeTimestamp(timestamp))
	kb.buf.WriteByte(Separator)
	kb.buf.WriteString(entityID)
	return kb.copyBytes()
}

// BuildPrefix constructs a prefix for scanning by entity type and field name.
func (kb *KeyBuilder) BuildPrefix(entityType, fieldName string) []byte {
	kb.Reset()
	kb.buf.WriteString(entityType)
	kb.buf.WriteByte(Separator)
	kb.buf.WriteString(fieldName)
	kb.buf.WriteByte(Separator)
	return kb.copyBytes()
}

// BuildValuePrefix constructs a prefix for scanning by entity type, field name, and value.
func (kb *KeyBuilder) BuildValuePrefix(entityType, fieldName string, value entity.Value) []byte {
	kb.Reset()
	kb.buf.WriteString(entityType)
	kb.buf.WriteByte(Separator)
	kb.buf.WriteString(fieldName)
	kb.buf.WriteByte(Separator)
	kb.buf.Write(EncodeValue(value))
	kb.buf.WriteByte(Separator)
	return kb.copyBytes()
}

// BuildRangeStart constructs a range start key for time-bounded queries.
func (kb *KeyBuilder) BuildRangeStart(entityType, fieldName string, value entity.Value, fromTimestamp int64) []byte {
	kb.Reset()
	kb.buf.WriteString(entityType)
	kb.buf.WriteByte(Separator)
	kb.buf.WriteString(fieldName)
	kb.buf.WriteByte(Separator)
	kb.buf.Write(EncodeValue(value))
	kb.buf.WriteByte(Separator)
	kb.buf.Write(encoding.EncodeTimestamp(fromTimestamp))
	kb.buf.WriteByte(Separator)
	return kb.copyBytes()
}

// BuildRangeEnd constructs a range end key for time-bounded queries.
func (kb *KeyBuilder) BuildRangeEnd(entityType, fieldName string, value entity.Value, toTimestamp int64) []byte {
	kb.Reset()
	kb.buf.WriteString(entityType)
	kb.buf.WriteByte(Separator)
	kb.buf.WriteString(fieldName)
	kb.buf.WriteByte(Separator)
	kb.buf.Write(EncodeValue(value))
	kb.buf.WriteByte(Separator)
	kb.buf.Write(encoding.EncodeTimestamp(toTimestamp))
	kb.buf.WriteByte(Separator)
	// Use 0xFF to ensure we include all entity IDs at this timestamp
	kb.buf.WriteByte(0xFF)
	return kb.copyBytes()
}

// BuildAllRangeStart constructs a range start for _all index queries.
func (kb *KeyBuilder) BuildAllRangeStart(entityType string, fromTimestamp int64) []byte {
	kb.Reset()
	kb.buf.WriteString(entityType)
	kb.buf.WriteByte(Separator)
	kb.buf.WriteString(AllFieldName)
	kb.buf.WriteByte(Separator)
	kb.buf.Write(encoding.EncodeTimestamp(fromTimestamp))
	kb.buf.WriteByte(Separator)
	return kb.copyBytes()
}

// BuildAllRangeEnd constructs a range end for _all index queries.
func (kb *KeyBuilder) BuildAllRangeEnd(entityType string, toTimestamp int64) []byte {
	kb.Reset()
	kb.buf.WriteString(entityType)
	kb.buf.WriteByte(Separator)
	kb.buf.WriteString(AllFieldName)
	kb.buf.WriteByte(Separator)
	kb.buf.Write(encoding.EncodeTimestamp(toTimestamp))
	kb.buf.WriteByte(Separator)
	kb.buf.WriteByte(0xFF)
	return kb.copyBytes()
}

// BuildByIDKey constructs a _by_id index key for entity version lookups.
// Format: {entity_type}/_by_id/{entity_id}/{timestamp_8_bytes}
func (kb *KeyBuilder) BuildByIDKey(entityType, entityID string, timestamp int64) []byte {
	kb.Reset()
	kb.buf.WriteString(entityType)
	kb.buf.WriteByte(Separator)
	kb.buf.WriteString(ByIDFieldName)
	kb.buf.WriteByte(Separator)
	kb.buf.WriteString(entityID)
	kb.buf.WriteByte(Separator)
	kb.buf.Write(encoding.EncodeTimestamp(timestamp))
	return kb.copyBytes()
}

// BuildByIDPrefix constructs a prefix for scanning all versions of an entity.
// Format: {entity_type}/_by_id/{entity_id}/
func (kb *KeyBuilder) BuildByIDPrefix(entityType, entityID string) []byte {
	kb.Reset()
	kb.buf.WriteString(entityType)
	kb.buf.WriteByte(Separator)
	kb.buf.WriteString(ByIDFieldName)
	kb.buf.WriteByte(Separator)
	kb.buf.WriteString(entityID)
	kb.buf.WriteByte(Separator)
	return kb.copyBytes()
}

// BuildByIDRangeStart constructs a range start for time-bounded _by_id queries.
func (kb *KeyBuilder) BuildByIDRangeStart(entityType, entityID string, fromTimestamp int64) []byte {
	kb.Reset()
	kb.buf.WriteString(entityType)
	kb.buf.WriteByte(Separator)
	kb.buf.WriteString(ByIDFieldName)
	kb.buf.WriteByte(Separator)
	kb.buf.WriteString(entityID)
	kb.buf.WriteByte(Separator)
	kb.buf.Write(encoding.EncodeTimestamp(fromTimestamp))
	return kb.copyBytes()
}

// BuildByIDRangeEnd constructs a range end for time-bounded _by_id queries.
func (kb *KeyBuilder) BuildByIDRangeEnd(entityType, entityID string, toTimestamp int64) []byte {
	kb.Reset()
	kb.buf.WriteString(entityType)
	kb.buf.WriteByte(Separator)
	kb.buf.WriteString(ByIDFieldName)
	kb.buf.WriteByte(Separator)
	kb.buf.WriteString(entityID)
	kb.buf.WriteByte(Separator)
	kb.buf.Write(encoding.EncodeTimestamp(toTimestamp))
	// Use 0xFF to ensure we include all entries at this timestamp
	kb.buf.WriteByte(0xFF)
	return kb.copyBytes()
}

func (kb *KeyBuilder) copyBytes() []byte {
	result := make([]byte, kb.buf.Len())
	copy(result, kb.buf.Bytes())
	return result
}

// EncodeValue encodes a Value to bytes for use in index keys.
func EncodeValue(v entity.Value) []byte {
	switch v.Kind {
	case entity.KindInt:
		return encoding.EncodeInt64(v.I)
	case entity.KindFloat:
		return encoding.EncodeFloat64(v.F)
	case entity.KindBool:
		return encoding.EncodeBool(v.B)
	case entity.KindString:
		return encoding.EncodeString(v.S)
	default:
		return nil
	}
}

// BuildIndexKeys builds all index keys for a value, handling array explosion.
func BuildIndexKeys(entityType, fieldName string, value entity.Value, timestamp int64, entityID string) [][]byte {
	kb := NewKeyBuilder()

	if value.Kind == entity.KindArray {
		// Explode array into multiple index entries
		keys := make([][]byte, 0, len(value.Arr))
		for _, elem := range value.Arr {
			keys = append(keys, kb.BuildKey(entityType, fieldName, elem, timestamp, entityID))
		}
		return keys
	}

	return [][]byte{kb.BuildKey(entityType, fieldName, value, timestamp, entityID)}
}

// ParseIndexKey extracts components from an index key.
// Returns entityType, fieldName, timestamp, entityID.
func ParseIndexKey(key []byte) (entityType, fieldName string, timestamp int64, entityID string) {
	parts := bytes.SplitN(key, []byte{Separator}, 5)
	if len(parts) < 5 {
		return
	}

	entityType = string(parts[0])
	fieldName = string(parts[1])
	// parts[2] is the encoded value (we skip it for parsing)
	if len(parts[3]) == 8 {
		timestamp = encoding.DecodeTimestamp(parts[3])
	}
	entityID = string(parts[4])
	return
}

// ParseAllIndexKey extracts components from an _all index key.
// Returns entityType, timestamp, entityID.
func ParseAllIndexKey(key []byte) (entityType string, timestamp int64, entityID string) {
	parts := bytes.SplitN(key, []byte{Separator}, 4)
	if len(parts) < 4 {
		return
	}

	entityType = string(parts[0])
	// parts[1] is "_all"
	if len(parts[2]) == 8 {
		timestamp = encoding.DecodeTimestamp(parts[2])
	}
	entityID = string(parts[3])
	return
}

// ParseByIDIndexKey extracts components from a _by_id index key.
// Format: {entity_type}/_by_id/{entity_id}/{timestamp_8_bytes}
// Returns entityType, entityID, timestamp.
func ParseByIDIndexKey(key []byte) (entityType, entityID string, timestamp int64) {
	parts := bytes.SplitN(key, []byte{Separator}, 4)
	if len(parts) < 4 {
		return
	}

	entityType = string(parts[0])
	// parts[1] is "_by_id"
	entityID = string(parts[2])
	if len(parts[3]) == 8 {
		timestamp = encoding.DecodeTimestamp(parts[3])
	}
	return
}
