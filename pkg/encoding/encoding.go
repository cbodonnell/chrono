package encoding

import (
	"encoding/binary"
	"math"
)

// EncodeInt64 encodes an int64 to a byte slice that maintains sort order.
// Uses big-endian encoding with sign bit XOR so negatives sort before positives.
func EncodeInt64(v int64) []byte {
	// XOR with sign bit to flip the ordering for negative numbers
	// This makes the byte representation sortable: negative < zero < positive
	u := uint64(v) ^ (1 << 63)
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, u)
	return buf
}

// DecodeInt64 decodes a byte slice back to int64.
func DecodeInt64(b []byte) int64 {
	u := binary.BigEndian.Uint64(b)
	return int64(u ^ (1 << 63))
}

// EncodeFloat64 encodes a float64 to a byte slice that maintains sort order.
// Uses IEEE 754 bit representation with sign fixup.
func EncodeFloat64(v float64) []byte {
	bits := math.Float64bits(v)

	// If the sign bit is set (negative number), flip all bits.
	// If positive, flip just the sign bit.
	// This ensures proper ordering: negative < zero < positive
	if bits&(1<<63) != 0 {
		bits = ^bits
	} else {
		bits ^= 1 << 63
	}

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, bits)
	return buf
}

// DecodeFloat64 decodes a byte slice back to float64.
func DecodeFloat64(b []byte) float64 {
	bits := binary.BigEndian.Uint64(b)

	// Reverse the transformation
	if bits&(1<<63) != 0 {
		bits ^= 1 << 63
	} else {
		bits = ^bits
	}

	return math.Float64frombits(bits)
}

// EncodeBool encodes a bool to a single byte.
func EncodeBool(v bool) []byte {
	if v {
		return []byte{0x01}
	}
	return []byte{0x00}
}

// DecodeBool decodes a byte slice back to bool.
func DecodeBool(b []byte) bool {
	return len(b) > 0 && b[0] != 0x00
}

// EncodeString encodes a string to bytes (UTF-8, already sortable).
func EncodeString(v string) []byte {
	return []byte(v)
}

// DecodeString decodes a byte slice back to string.
func DecodeString(b []byte) string {
	return string(b)
}

// EncodeTimestamp encodes a Unix nanosecond timestamp (int64) for index keys.
// Uses the same encoding as int64 to maintain sort order.
func EncodeTimestamp(ts int64) []byte {
	return EncodeInt64(ts)
}

// DecodeTimestamp decodes a timestamp from bytes.
func DecodeTimestamp(b []byte) int64 {
	return DecodeInt64(b)
}
