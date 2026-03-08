package entity

import (
	"testing"
)

func TestParsePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantSegs int
		wantErr  bool
	}{
		{
			name:     "simple field",
			input:    "field",
			wantSegs: 1,
			wantErr:  false,
		},
		{
			name:     "nested two levels",
			input:    "a.b",
			wantSegs: 2,
			wantErr:  false,
		},
		{
			name:     "nested three levels",
			input:    "a.b.c",
			wantSegs: 3,
			wantErr:  false,
		},
		{
			name:     "array index",
			input:    "items[0]",
			wantSegs: 1,
			wantErr:  false,
		},
		{
			name:     "array with nested field",
			input:    "items[0].name",
			wantSegs: 2,
			wantErr:  false,
		},
		{
			name:     "complex path",
			input:    "a.b[0].c[1].d",
			wantSegs: 4,
			wantErr:  false,
		},
		{
			name:     "deep nesting",
			input:    "field.items[2].name",
			wantSegs: 3,
			wantErr:  false,
		},
		{
			name:     "empty path",
			input:    "",
			wantSegs: 0,
			wantErr:  true,
		},
		{
			name:     "trailing dot",
			input:    "field.",
			wantSegs: 0,
			wantErr:  true,
		},
		{
			name:     "unclosed bracket",
			input:    "field[0",
			wantSegs: 0,
			wantErr:  true,
		},
		{
			name:     "empty bracket",
			input:    "field[]",
			wantSegs: 0,
			wantErr:  true,
		},
		{
			name:     "negative index",
			input:    "field[-1]",
			wantSegs: 0,
			wantErr:  true,
		},
		{
			name:     "non-numeric index",
			input:    "field[abc]",
			wantSegs: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := ParsePath(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParsePath(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ParsePath(%q) unexpected error: %v", tt.input, err)
				return
			}
			if len(path) != tt.wantSegs {
				t.Errorf("ParsePath(%q) got %d segments, want %d", tt.input, len(path), tt.wantSegs)
			}
		})
	}
}

func TestParsePathSegments(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []PathSegment
	}{
		{
			name:  "simple field",
			input: "field",
			want: []PathSegment{
				{Field: "field", Index: nil},
			},
		},
		{
			name:  "array index",
			input: "items[0]",
			want: []PathSegment{
				{Field: "items", Index: intPtr(0)},
			},
		},
		{
			name:  "nested path",
			input: "metadata.location",
			want: []PathSegment{
				{Field: "metadata", Index: nil},
				{Field: "location", Index: nil},
			},
		},
		{
			name:  "array with nested",
			input: "items[2].name",
			want: []PathSegment{
				{Field: "items", Index: intPtr(2)},
				{Field: "name", Index: nil},
			},
		},
		{
			name:  "complex path",
			input: "a.b[0].c",
			want: []PathSegment{
				{Field: "a", Index: nil},
				{Field: "b", Index: intPtr(0)},
				{Field: "c", Index: nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := ParsePath(tt.input)
			if err != nil {
				t.Fatalf("ParsePath(%q) error: %v", tt.input, err)
			}
			if len(path) != len(tt.want) {
				t.Fatalf("ParsePath(%q) got %d segments, want %d", tt.input, len(path), len(tt.want))
			}
			for i, seg := range path {
				if seg.Field != tt.want[i].Field {
					t.Errorf("segment %d: Field = %q, want %q", i, seg.Field, tt.want[i].Field)
				}
				if (seg.Index == nil) != (tt.want[i].Index == nil) {
					t.Errorf("segment %d: Index nil mismatch", i)
				} else if seg.Index != nil && *seg.Index != *tt.want[i].Index {
					t.Errorf("segment %d: Index = %d, want %d", i, *seg.Index, *tt.want[i].Index)
				}
			}
		})
	}
}

func TestPathExtract(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		fields map[string]Value
		want   Value
		wantOK bool
	}{
		{
			name: "simple field",
			path: "status",
			fields: map[string]Value{
				"status": NewString("active"),
			},
			want:   NewString("active"),
			wantOK: true,
		},
		{
			name: "nested object field",
			path: "metadata.location",
			fields: map[string]Value{
				"metadata": NewObject(map[string]Value{
					"location": NewString("building-a"),
					"owner":    NewString("team-x"),
				}),
			},
			want:   NewString("building-a"),
			wantOK: true,
		},
		{
			name: "deep nested field",
			path: "a.b.c",
			fields: map[string]Value{
				"a": NewObject(map[string]Value{
					"b": NewObject(map[string]Value{
						"c": NewInt(42),
					}),
				}),
			},
			want:   NewInt(42),
			wantOK: true,
		},
		{
			name: "array index",
			path: "items[0]",
			fields: map[string]Value{
				"items": NewArray([]Value{
					NewString("first"),
					NewString("second"),
				}),
			},
			want:   NewString("first"),
			wantOK: true,
		},
		{
			name: "array with nested field",
			path: "readings[0].value",
			fields: map[string]Value{
				"readings": NewArray([]Value{
					NewObject(map[string]Value{
						"value": NewFloat(72.5),
						"unit":  NewString("F"),
					}),
					NewObject(map[string]Value{
						"value": NewFloat(23.1),
						"unit":  NewString("C"),
					}),
				}),
			},
			want:   NewFloat(72.5),
			wantOK: true,
		},
		{
			name: "second array element",
			path: "readings[1].unit",
			fields: map[string]Value{
				"readings": NewArray([]Value{
					NewObject(map[string]Value{
						"value": NewFloat(72.5),
						"unit":  NewString("F"),
					}),
					NewObject(map[string]Value{
						"value": NewFloat(23.1),
						"unit":  NewString("C"),
					}),
				}),
			},
			want:   NewString("C"),
			wantOK: true,
		},
		{
			name:   "missing top-level field",
			path:   "nonexistent",
			fields: map[string]Value{},
			want:   Value{},
			wantOK: false,
		},
		{
			name: "missing nested field",
			path: "metadata.missing",
			fields: map[string]Value{
				"metadata": NewObject(map[string]Value{
					"location": NewString("building-a"),
				}),
			},
			want:   Value{},
			wantOK: false,
		},
		{
			name: "array index out of bounds",
			path: "items[5]",
			fields: map[string]Value{
				"items": NewArray([]Value{
					NewString("first"),
				}),
			},
			want:   Value{},
			wantOK: false,
		},
		{
			name: "not an array",
			path: "field[0]",
			fields: map[string]Value{
				"field": NewString("not an array"),
			},
			want:   Value{},
			wantOK: false,
		},
		{
			name: "not an object",
			path: "field.child",
			fields: map[string]Value{
				"field": NewString("not an object"),
			},
			want:   Value{},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := ParsePath(tt.path)
			if err != nil {
				t.Fatalf("ParsePath(%q) error: %v", tt.path, err)
			}

			got, ok := path.Extract(tt.fields)
			if ok != tt.wantOK {
				t.Errorf("Extract() ok = %v, want %v", ok, tt.wantOK)
				return
			}
			if !tt.wantOK {
				return
			}

			if got.Kind != tt.want.Kind {
				t.Errorf("Extract() Kind = %v, want %v", got.Kind, tt.want.Kind)
				return
			}

			switch got.Kind {
			case KindInt:
				if got.I != tt.want.I {
					t.Errorf("Extract() Int = %d, want %d", got.I, tt.want.I)
				}
			case KindFloat:
				if got.F != tt.want.F {
					t.Errorf("Extract() Float = %f, want %f", got.F, tt.want.F)
				}
			case KindBool:
				if got.B != tt.want.B {
					t.Errorf("Extract() Bool = %v, want %v", got.B, tt.want.B)
				}
			case KindString:
				if got.S != tt.want.S {
					t.Errorf("Extract() String = %q, want %q", got.S, tt.want.S)
				}
			}
		})
	}
}

func TestPathString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"field", "field"},
		{"a.b", "a.b"},
		{"items[0]", "items[0]"},
		{"a.b[0].c", "a.b[0].c"},
		{"metadata.location", "metadata.location"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			path, err := ParsePath(tt.input)
			if err != nil {
				t.Fatalf("ParsePath(%q) error: %v", tt.input, err)
			}
			got := path.String()
			if got != tt.want {
				t.Errorf("Path.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func intPtr(i int) *int {
	return &i
}
