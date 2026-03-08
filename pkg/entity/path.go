package entity

import (
	"fmt"
	"strconv"
	"strings"
)

// PathSegment represents one step in a field path.
type PathSegment struct {
	Field string // Field name (empty if pure array access after another array)
	Index *int   // Array index, nil if not an array access
}

// Path is a parsed field path.
type Path []PathSegment

// ParsePath parses a dot-notation path like "field.child[0].name" into segments.
func ParsePath(s string) (Path, error) {
	if s == "" {
		return nil, fmt.Errorf("empty path")
	}

	var segments []PathSegment
	remaining := s

	for remaining != "" {
		seg, rest, err := parseNextSegment(remaining)
		if err != nil {
			return nil, fmt.Errorf("invalid path %q: %w", s, err)
		}
		segments = append(segments, seg)
		remaining = rest
	}

	return segments, nil
}

// parseNextSegment parses the next segment from the path string.
// Returns the segment, remaining string, and any error.
func parseNextSegment(s string) (PathSegment, string, error) {
	var seg PathSegment

	// Handle leading dot (skip it)
	if strings.HasPrefix(s, ".") {
		s = s[1:]
		if s == "" {
			return seg, "", fmt.Errorf("trailing dot")
		}
	}

	// Check if we start with a bracket (array access without field name)
	if strings.HasPrefix(s, "[") {
		idx, rest, err := parseArrayIndex(s)
		if err != nil {
			return seg, "", err
		}
		seg.Index = &idx
		return seg, rest, nil
	}

	// Parse field name (until we hit '.', '[', or end of string)
	fieldEnd := strings.IndexAny(s, ".[")
	if fieldEnd == -1 {
		// Rest of string is the field name
		seg.Field = s
		return seg, "", nil
	}

	if fieldEnd == 0 {
		return seg, "", fmt.Errorf("empty field name")
	}

	seg.Field = s[:fieldEnd]
	rest := s[fieldEnd:]

	// Check if followed by array index
	if strings.HasPrefix(rest, "[") {
		idx, rest2, err := parseArrayIndex(rest)
		if err != nil {
			return seg, "", err
		}
		seg.Index = &idx
		return seg, rest2, nil
	}

	return seg, rest, nil
}

// parseArrayIndex parses "[N]" and returns the index and remaining string.
func parseArrayIndex(s string) (int, string, error) {
	if !strings.HasPrefix(s, "[") {
		return 0, "", fmt.Errorf("expected '['")
	}

	end := strings.Index(s, "]")
	if end == -1 {
		return 0, "", fmt.Errorf("unclosed bracket")
	}

	idxStr := s[1:end]
	if idxStr == "" {
		return 0, "", fmt.Errorf("empty array index")
	}

	idx, err := strconv.Atoi(idxStr)
	if err != nil {
		return 0, "", fmt.Errorf("invalid array index %q: %w", idxStr, err)
	}

	if idx < 0 {
		return 0, "", fmt.Errorf("negative array index: %d", idx)
	}

	return idx, s[end+1:], nil
}

// Extract retrieves a value from a fields map following the path.
// Returns the value and true if found, or zero value and false if not.
func (p Path) Extract(fields map[string]Value) (Value, bool) {
	if len(p) == 0 {
		return Value{}, false
	}

	// Start with the first segment from the fields map
	seg := p[0]

	var current Value
	var ok bool

	if seg.Field != "" {
		current, ok = fields[seg.Field]
		if !ok {
			return Value{}, false
		}
	} else {
		// Path starts with array index, which is invalid for top-level
		return Value{}, false
	}

	// Apply array index if present
	if seg.Index != nil {
		if current.Kind != KindArray {
			return Value{}, false
		}
		idx := *seg.Index
		if idx >= len(current.Arr) {
			return Value{}, false
		}
		current = current.Arr[idx]
	}

	// Process remaining segments
	for _, seg := range p[1:] {
		// Access field if specified
		if seg.Field != "" {
			if current.Kind != KindObject {
				return Value{}, false
			}
			current, ok = current.Obj[seg.Field]
			if !ok {
				return Value{}, false
			}
		}

		// Apply array index if present
		if seg.Index != nil {
			if current.Kind != KindArray {
				return Value{}, false
			}
			idx := *seg.Index
			if idx >= len(current.Arr) {
				return Value{}, false
			}
			current = current.Arr[idx]
		}
	}

	return current, true
}

// String returns the original path string representation.
func (p Path) String() string {
	var sb strings.Builder
	for i, seg := range p {
		if seg.Field != "" {
			if i > 0 {
				sb.WriteByte('.')
			}
			sb.WriteString(seg.Field)
		}
		if seg.Index != nil {
			sb.WriteByte('[')
			sb.WriteString(strconv.Itoa(*seg.Index))
			sb.WriteByte(']')
		}
	}
	return sb.String()
}
