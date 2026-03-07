package encoding

import (
	"bytes"
	"math"
	"sort"
	"testing"
)

func TestEncodeDecodeInt64(t *testing.T) {
	tests := []int64{
		math.MinInt64,
		-1000000,
		-1,
		0,
		1,
		1000000,
		math.MaxInt64,
	}

	for _, v := range tests {
		encoded := EncodeInt64(v)
		decoded := DecodeInt64(encoded)
		if decoded != v {
			t.Errorf("Int64 roundtrip failed: got %d, want %d", decoded, v)
		}
	}
}

func TestInt64SortOrder(t *testing.T) {
	values := []int64{math.MaxInt64, -1, 0, 1000, math.MinInt64, -1000, 1}

	type pair struct {
		val     int64
		encoded []byte
	}
	pairs := make([]pair, len(values))
	for i, v := range values {
		pairs[i] = pair{val: v, encoded: EncodeInt64(v)}
	}

	// Sort by encoded bytes
	sort.Slice(pairs, func(i, j int) bool {
		return bytes.Compare(pairs[i].encoded, pairs[j].encoded) < 0
	})

	// Verify sort order matches numeric order
	for i := 1; i < len(pairs); i++ {
		if pairs[i-1].val >= pairs[i].val {
			t.Errorf("Sort order incorrect at position %d: %d >= %d", i, pairs[i-1].val, pairs[i].val)
		}
	}
}

func TestEncodeDecodeFloat64(t *testing.T) {
	tests := []float64{
		math.Inf(-1),
		-math.MaxFloat64,
		-1000000.5,
		-1.0,
		-math.SmallestNonzeroFloat64,
		0,
		math.SmallestNonzeroFloat64,
		1.0,
		1000000.5,
		math.MaxFloat64,
		math.Inf(1),
	}

	for _, v := range tests {
		encoded := EncodeFloat64(v)
		decoded := DecodeFloat64(encoded)
		if decoded != v {
			t.Errorf("Float64 roundtrip failed: got %v, want %v", decoded, v)
		}
	}
}

func TestFloat64SortOrder(t *testing.T) {
	values := []float64{
		math.Inf(1),
		0,
		-1.5,
		100.0,
		math.Inf(-1),
		-100.0,
		1.5,
	}
	encoded := make([][]byte, len(values))

	for i, v := range values {
		encoded[i] = EncodeFloat64(v)
	}

	// Sort by encoded bytes
	indices := make([]int, len(values))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool {
		return bytes.Compare(encoded[indices[i]], encoded[indices[j]]) < 0
	})

	sorted := make([]float64, len(values))
	for i, idx := range indices {
		sorted[i] = values[idx]
	}

	// Verify sort order matches numeric order
	for i := 1; i < len(sorted); i++ {
		if sorted[i-1] >= sorted[i] {
			t.Errorf("Sort order incorrect at position %d: %v >= %v", i, sorted[i-1], sorted[i])
		}
	}
}

func TestEncodeDecodeBool(t *testing.T) {
	falseEncoded := EncodeBool(false)
	trueEncoded := EncodeBool(true)

	if DecodeBool(falseEncoded) != false {
		t.Error("Bool false roundtrip failed")
	}
	if DecodeBool(trueEncoded) != true {
		t.Error("Bool true roundtrip failed")
	}

	// false should sort before true
	if bytes.Compare(falseEncoded, trueEncoded) >= 0 {
		t.Error("Bool sort order incorrect: false should be < true")
	}
}

func TestEncodeDecodeString(t *testing.T) {
	tests := []string{
		"",
		"a",
		"hello",
		"hello world",
		"こんにちは",
	}

	for _, v := range tests {
		encoded := EncodeString(v)
		decoded := DecodeString(encoded)
		if decoded != v {
			t.Errorf("String roundtrip failed: got %q, want %q", decoded, v)
		}
	}
}

func TestStringSortOrder(t *testing.T) {
	values := []string{"banana", "apple", "cherry", "Apple"}
	encoded := make([][]byte, len(values))

	for i, v := range values {
		encoded[i] = EncodeString(v)
	}

	// Sort by encoded bytes
	indices := make([]int, len(values))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool {
		return bytes.Compare(encoded[indices[i]], encoded[indices[j]]) < 0
	})

	sorted := make([]string, len(values))
	for i, idx := range indices {
		sorted[i] = values[idx]
	}

	// Should be lexicographic order (case-sensitive)
	expected := []string{"Apple", "apple", "banana", "cherry"}
	for i, v := range sorted {
		if v != expected[i] {
			t.Errorf("String sort order incorrect at position %d: got %q, want %q", i, v, expected[i])
		}
	}
}
