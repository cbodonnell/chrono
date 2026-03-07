# Universal Entity Store — Database System Design

## Core Requirements

| Requirement | Notes |
|---|---|
| Any entity type, any fields | Schema-free / dynamic schema |
| Primitive Go types | `int64`, `float64`, `bool`, `string`, `[]T` |
| Configurable field indexes | Per-entity-type index config |
| Full entity retrieval by ID | K-V lookup |
| Time-series querying | Efficient range scans over time |

---

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Write Path                             │
│  Entity (Go struct / map) → Serializer → Fanout Writer      │
│                                    ↓              ↓         │
│                              KV Store     Index Store       │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                      Read Path                              │
│  Query (field filters + time range) → Index Planner         │
│       ↓ entity IDs                                          │
│  KV Store fetch → Deserialize → Return entities             │
└─────────────────────────────────────────────────────────────┘
```

---

## Storage Layer: Two Complementary Stores

### 1. KV Store — Full Entity Retrieval

**Technology:** BadgerDB (embedded) or Redis / TiKV for distributed.

**Key schema:**
```
{entity_type}:{entity_id}  →  msgpack/protobuf blob of full entity
```

Example:
```
sensor:abc-123  →  {id: "abc-123", temp: 72.4, active: true, tags: ["A","B"], ts: 1709734800}
```

- Writes are O(1)
- Reads by ID are O(1)
- No schema needed — blob is opaque to the KV layer

---

### 2. Index Store — Queryable Fields + Time-Series

**Technology:** RocksDB / BadgerDB (with careful key design), or ClickHouse for large-scale analytical workloads.

The key insight: **encode indexed field values directly into the key**, so range scans become sequential reads.

#### Key Schema (Composite Key Design)

```
{entity_type} / {field_name} / {field_value_bytes} / {timestamp_unix_ns} / {entity_id}
→  (empty value, or pointer back to KV)
```

Examples:
```
sensor/active/\x01/1709734800000000000/abc-123        → ""
sensor/temp/\x40\x52\x19\x99.../1709734800000000000/abc-123  → ""
sensor/tags/A/1709734800000000000/abc-123             → ""
sensor/tags/B/1709734800000000000/abc-123             → ""
```

**Why this works:**
- **Field filter:** Prefix scan on `{type}/{field}/{value}/` gives all matching entities
- **Time range:** Timestamp in the key enables efficient time-series slices via prefix + range scan
- **Compound queries:** Intersect result sets from multiple index scans (or use bloom filters to short-circuit)

#### Value Encoding for Sorted Scans

For numeric types, use **order-preserving encoding** so lexicographic key order equals numeric order:

| Go type | Encoding |
|---|---|
| `int64` | Big-endian, XOR sign bit (negatives sort before positives) |
| `float64` | IEEE 754 bit-reinterpret + sign fixup |
| `bool` | `\x00` / `\x01` |
| `string` | Raw UTF-8 (already lexicographically sortable) |
| `[]T` | Explode into one index entry per element |

---

## Schema / Index Configuration

Define indexes per entity type in config (YAML, DB table, or code):

```yaml
entity_types:
  sensor:
    indexes:
      - field: temp
        type: float64
      - field: active
        type: bool
      - field: tags
        type: []string   # array → exploded index entries
    ttl: 90d

  order:
    indexes:
      - field: user_id
        type: string
      - field: amount
        type: float64
      - field: status
        type: string
```

At write time, the **Index Configuration Registry** determines which fields get index entries written. Non-indexed fields are still stored in the KV blob — they're just not directly queryable (post-filter in application code).

---

## Entity Schema in Go

```go
// The universal entity — schema-free
type Entity struct {
    ID         string            `msgpack:"id"`
    Type       string            `msgpack:"type"`
    Timestamp  int64             `msgpack:"ts"`   // Unix nanoseconds — primary time axis
    Fields     map[string]Value  `msgpack:"f"`
}

// Discriminated union covering all Go primitive types + arrays
type Value struct {
    Kind ValueKind   `msgpack:"k"`
    I    int64       `msgpack:"i,omitempty"`
    F    float64     `msgpack:"f,omitempty"`
    B    bool        `msgpack:"b,omitempty"`
    S    string      `msgpack:"s,omitempty"`
    Arr  []Value     `msgpack:"a,omitempty"`
}

type ValueKind uint8
const (
    KindInt ValueKind = iota
    KindFloat
    KindBool
    KindString
    KindArray
)
```

---

## Write Path

```go
func (store *EntityStore) Write(e Entity) error {
    // 1. Serialize full entity → KV
    blob, _ := msgpack.Marshal(e)
    kvKey := fmt.Sprintf("%s:%s", e.Type, e.ID)
    store.kv.Set(kvKey, blob)

    // 2. Look up index config for this entity type
    cfg := store.indexRegistry.Get(e.Type)

    // 3. Write one index entry per indexed field
    for _, idxField := range cfg.Indexes {
        val, ok := e.Fields[idxField.Name]
        if !ok { continue }

        for _, key := range buildIndexKeys(e.Type, idxField.Name, val, e.Timestamp, e.ID) {
            store.indexDB.Set(key, nil)
        }
    }
    return nil
}
```

---

## Query Path

```go
type Query struct {
    EntityType string
    Filters    []FieldFilter  // AND semantics
    TimeRange  *TimeRange
    Limit      int
}

type FieldFilter struct {
    Field string
    Op    Op       // Eq, Lt, Gt, In, Contains (for arrays)
    Value Value
}

type TimeRange struct {
    From, To int64  // Unix nanoseconds
}
```

**Execution:**
1. For each filter, perform an index range scan → get a set of `(timestamp, entity_id)` pairs
2. Intersect sets (smallest first, like Postgres bitmap index AND)
3. For each surviving entity ID, fetch full blob from KV store
4. Optional post-filter for non-indexed fields

---

## Time-Series Query

Because timestamp is embedded in every index key, time-series querying comes for free:

```
Scan prefix: sensor/temp/\x40\x52...  (value = 72.4)
  From key:  sensor/temp/.../1709600000000000000/\x00  (start of time range)
  To key:    sensor/temp/.../1709820000000000000/\xFF  (end of time range)
```

This returns all `sensor` entities where `temp = 72.4` in a given time window, in **chronological order**, with no sort step needed.

For pure time-series across all entities of a type, add a synthetic `_all` index entry per write, keyed only by timestamp:

```
sensor/_all/1709734800000000000/abc-123  → ""
```

---

## Technology Recommendations

| Scale | KV Store | Index Store | Notes |
|---|---|---|---|
| Single-node / embedded | BadgerDB | BadgerDB (key prefix separation) | Simple ops, great for Go |
| Mid-scale | Redis | RocksDB or BadgerDB | Redis for low-latency ID lookup |
| Large-scale analytical | S3 + DynamoDB | ClickHouse | ClickHouse natively handles time-series + columnar indexes |
| Distributed OLTP | TiKV | TiKV | Supports ordered key scans natively |

**For a pure Go embedded solution:** BadgerDB is the strongest fit — LSM-tree based (great for write throughput and range scans), actively maintained, and has prefix iteration built in.

---

## Key Design Tradeoffs

**Array denormalization:** Each element in a `[]string` gets its own index key. Querying `tags contains "A"` is a single prefix scan, not a table scan. Write amplification is bounded by array cardinality.

**Timestamp granularity:** Nanoseconds in the key gives sub-millisecond time-series resolution. If second-level resolution is sufficient, use Unix seconds to shrink key sizes.

**No composite indexes by default:** Each field is indexed independently; result sets are intersected in memory. If compound indexes are needed (e.g., `status=active AND region=us-east`), add an explicit second key schema:
```
{type}/{field1}+{field2}/{v1}+{v2}/{ts}/{id}
```
This must be configured explicitly to avoid combinatorial explosion.

**TTL / retention:** BadgerDB and RocksDB both support per-key TTL natively. Set TTL on index keys matching your `ttl` config. KV entries can carry an independent, longer-lived TTL.
