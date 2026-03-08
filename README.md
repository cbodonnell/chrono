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
go install github.com/cbodonnell/chrono/cmd/server@latest
```

Or build from source:

```bash
git clone https://github.com/cbodonnell/chrono.git
cd chrono
go build -o chrono ./cmd/server
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
  "limit": 100
}
```

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

Full entities are stored as serialized JSON blobs with O(1) access:

```
{entity_type}:{entity_id} → serialized entity blob
```

### Index Store

Index keys encode field values and timestamps for efficient range scans:

```
{entity_type}/{field_name}/{encoded_value}/{timestamp_ns}/{entity_id} → (empty)
```

A synthetic `_all` index enables pure time-series queries:

```
{entity_type}/_all/{timestamp_ns}/{entity_id} → (empty)
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

## License

MIT
