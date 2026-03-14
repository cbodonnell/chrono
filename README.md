# Chrono

A lightweight schema-free entity database with configurable field-level indexes, efficient time-series range queries, and O(1) entity retrieval.

## Features

- **Schema-free entities** — Store any entity type with dynamic fields
- **Configurable indexes** — Define which fields are indexed per entity type via YAML config
- **Time-series queries** — Efficient range scans with timestamps embedded in index keys
- **O(1) entity retrieval** — Full entities stored in key-value store
- **Persistent storage** — BadgerDB-backed for durability
- **HTTP API** — Simple REST interface for all operations

## Installation

```bash
go install github.com/cbodonnell/chrono/cmd/chrono@latest
```

Or build from source:

```bash
git clone https://github.com/cbodonnell/chrono.git
cd chrono
go build -o chrono ./cmd/chrono
```

## Quick Start

1. Create a configuration file `chrono.yaml`:

```yaml
server:
  addr: ":8080"

storage:
  data_dir: "./data"

entities:
  sensor:
    indexes:
      - name: temp
        type: float
      - name: active
        type: bool
      - name: metadata.location
        type: string
      - name: tags
        type: string_array
```

2. Start the server:

```bash
./chrono -config chrono.yaml
```

3. Write an entity:

```bash
curl -X POST http://localhost:8080/entities \
  -H "Content-Type: application/json" \
  -d '{
    "id": "sensor-001",
    "type": "sensor",
    "fields": {
      "temp": 72.5,
      "active": true,
      "metadata": {
        "location": "floor-1",
        "owner": "team-a"
      },
      "tags": ["production", "critical"]
    }
  }'
```

4. Query entities:

```bash
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -d '{
    "entity_type": "sensor",
    "filters": [
      {"field": "metadata.location", "op": "eq", "value": "floor-1"}
    ]
  }'
```

## Configuration

Entity types and their indexes are defined in the YAML configuration file:

```yaml
server:
  addr: ":8080"              # HTTP listen address (default: ":8080")

storage:
  data_dir: "./data"         # Data directory for BadgerDB (default: "./data")

entities:
  sensor:                    # Entity type name
    indexes:
      - name: temp           # Field name
        type: float          # Field type
      - name: active
        type: bool
      - name: tags
        type: string_array

  order:
    indexes:
      - name: user_id
        type: string
      - name: amount
        type: float
      - name: status
        type: string
```

### Supported Field Types

| Type | Description |
|------|-------------|
| `int` / `int64` | 64-bit signed integer |
| `float` / `float64` | 64-bit floating point |
| `bool` | Boolean |
| `string` | UTF-8 string |
| `string_array` / `[]string` | Array of strings |
| `int_array` / `[]int` | Array of integers |
| `float_array` / `[]float` | Array of floats |

### Nested Field Paths

Index names support dot-notation for nested objects and bracket notation for arrays:

| Path | Description |
|------|-------------|
| `field` | Top-level field |
| `field.child` | Nested object field |
| `field[0]` | Array element |
| `field[0].child` | Array element's nested field |

## HTTP API

### Create/Update Entity

```
POST /entities
```

```json
{
  "id": "sensor-001",
  "type": "sensor",
  "timestamp": 1704067200000000000,
  "fields": {
    "temp": 72.5,
    "active": true,
    "metadata": {
      "location": "floor-1",
      "owner": "team-a"
    },
    "tags": ["production"]
  }
}
```

- `id` (required): Unique entity identifier
- `type` (required): Entity type (must be configured in config file)
- `timestamp` (optional): Unix nanoseconds; defaults to current time
- `fields`: Key-value map of entity data

### Get Entity

```
GET /entities/{type}/{id}
```

Returns the latest version of the entity.

### Get Entity History

```
GET /entities/{type}/{id}/history?from=<timestamp>&to=<timestamp>&limit=<n>&reverse=true&cursor=<timestamp>
```

Returns versions of an entity with cursor-based pagination.

| Parameter | Description |
|-----------|-------------|
| `from` | Start of time range (Unix nanoseconds, inclusive) |
| `to` | End of time range (Unix nanoseconds, inclusive) |
| `limit` | Maximum versions to return (default: 100) |
| `reverse` | If `true`, return newest versions first |
| `cursor` | Pagination cursor (timestamp from previous response's `next`) |

Response:

```json
{
  "items": [...],
  "next": "1704067200000000000"
}
```

The `next` field is only present if there are more results.

### Delete Entity

```
DELETE /entities/{type}/{id}
```

### Query Entities

```
POST /query
```

```json
{
  "entity_type": "sensor",
  "filters": [
    {"field": "active", "op": "eq", "value": true},
    {"field": "metadata.location", "op": "eq", "value": "floor-1"},
    {"field": "tags", "op": "contains", "value": "production"}
  ],
  "time_range": {
    "from": 1704067200000000000,
    "to": 1704153600000000000
  },
  "limit": 100,
  "reverse": false,
  "all_versions": false
}
```

| Field | Description |
|-------|-------------|
| `entity_type` | Required. Entity type to query |
| `filters` | Filter conditions |
| `time_range` | Optional time bounds (Unix nanoseconds) |
| `limit` | Maximum results to return |
| `reverse` | If `true`, return newest first |
| `all_versions` | If `false` (default), return only entities whose latest version matches. If `true`, return all matching historical versions |
| `match_any` | If `false` (default), use AND semantics (match all filters). If `true`, use OR semantics (match any filter) |

### Filter Operations

| Operation | Aliases | Description |
|-----------|---------|-------------|
| `eq` | `=`, `==` | Exact equality |
| `lt` | `<` | Less than |
| `lte` | `<=` | Less than or equal |
| `gt` | `>` | Greater than |
| `gte` | `>=` | Greater than or equal |
| `contains` | | Array contains element |

Multiple filters use AND semantics.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Write Path                             │
│  HTTP Request → JSON Decode → Serializer → Fanout Writer    │
│                                           ↓           ↓     │
│                                      KV Store    Index Store│
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                      Read Path                              │
│  HTTP Request → Query (filters + time range) → Index Scan   │
│       ↓ entity IDs                                          │
│  KV Store fetch → Deserialize → JSON Response               │
└─────────────────────────────────────────────────────────────┘
```

## Storage Design

### KV Store

Full entities are stored as serialized blobs. Each version is stored separately, keyed by timestamp:

```
{entity_type}:{entity_id}:{timestamp_hex} → serialized entity blob
```

This versioned design preserves full history and enables efficient version lookups.

### Index Store

Index keys encode field values and timestamps for efficient range scans:

```
{entity_type}/{field_name}/{encoded_value}/{timestamp_ns}/{entity_id} → (empty)
```

Synthetic indexes enable efficient access patterns:

```
{entity_type}/_all/{timestamp_ns}/{entity_id} → (empty)      # time-series queries
{entity_type}/_by_id/{entity_id}/{timestamp_ns} → (empty)    # version history
{entity_type}/_latest/{field}/{value}/{entity_id} → (empty)  # current state queries
{entity_type}/_latest_all/{entity_id} → (empty)              # all current entities
```

### Value Encoding

Numeric types use order-preserving encoding so lexicographic byte order equals numeric order:

| Type | Encoding |
|------|----------|
| `int64` | Big-endian with sign bit XOR |
| `float64` | IEEE 754 bits with sign fixup |
| `bool` | `0x00` / `0x01` |
| `string` | Raw UTF-8 |
| `[]T` | Exploded into one index entry per element |

## Query Examples

**Find all active sensors:**

```bash
curl -X POST http://localhost:8080/query -d '{
  "entity_type": "sensor",
  "filters": [{"field": "active", "op": "eq", "value": true}]
}'
```

**Find sensors in the last hour:**

```bash
curl -X POST http://localhost:8080/query -d '{
  "entity_type": "sensor",
  "time_range": {
    "from": '$(date -v-1H +%s)000000000',
    "to": '$(date +%s)000000000'
  }
}'
```

**Find sensors with temperature > 70 and tag "production":**

```bash
curl -X POST http://localhost:8080/query -d '{
  "entity_type": "sensor",
  "filters": [
    {"field": "temp", "op": "gt", "value": 70},
    {"field": "tags", "op": "contains", "value": "production"}
  ],
  "limit": 50
}'
```

## Embedding

Chrono can be embedded directly in your Go application:

```go
import (
    "github.com/cbodonnell/chrono/pkg/entity"
    "github.com/cbodonnell/chrono/pkg/index"
    "github.com/cbodonnell/chrono/pkg/store"
    badgerstore "github.com/cbodonnell/chrono/pkg/store/badger"
    "github.com/dgraph-io/badger/v4"
)

// Open BadgerDB
db, _ := badger.Open(badger.DefaultOptions("./data"))
defer db.Close()

// Configure indexes
registry := index.NewRegistry()
registry.Register("sensor", &index.EntityTypeConfig{
    Indexes: []index.FieldIndex{
        {Name: "temp", Type: index.FieldTypeFloat},
        {Name: "active", Type: index.FieldTypeBool},
    },
})

// Create store
es := store.NewEntityStore(
    badgerstore.NewKVStore(db),
    badgerstore.NewIndexStore(db),
    registry,
    store.NewMsgpackSerializer(),
)
defer es.Close()

// Use it
es.Write(&entity.Entity{...})
results, _ := es.Query(&store.Query{...})
```

## Future Work

- Partitioning by time for large datasets
- Composite field indexes and bitmap indexes for high-cardinality fields
- Temporal validity (_start and _end instead of a single timestamp)
  - _start = _end for point-in-time entities
  - _start < _end for entities with a known time range
  - _end = ∞ for entities with open-ended validity (e.g. user sessions)

## License

MIT
