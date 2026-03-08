// Package main demonstrates chrono as an Application Performance Monitoring (APM) store.
//
// This example shows how chrono excels at storing and querying time-series request data
// with efficient range queries, multi-filter searches, and tag-based categorization.
//
// Run with: go run main.go
package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/cbodonnell/chrono/pkg/entity"
	"github.com/cbodonnell/chrono/pkg/index"
	"github.com/cbodonnell/chrono/pkg/store"
	badgerstore "github.com/cbodonnell/chrono/pkg/store/badger"
	"github.com/dgraph-io/badger/v4"
)

const entityType = "http_request"

// Services and endpoints to simulate
var services = []string{"api-gateway", "user-service", "order-service", "payment-service"}
var endpoints = map[string][]string{
	"api-gateway":     {"/v1/health", "/v1/users", "/v1/orders", "/v1/payments"},
	"user-service":    {"/users", "/users/auth", "/users/profile", "/users/settings"},
	"order-service":   {"/orders", "/orders/create", "/orders/status", "/orders/history"},
	"payment-service": {"/payments", "/payments/process", "/payments/refund", "/payments/verify"},
}
var methods = []string{"GET", "POST", "PUT", "DELETE"}

func main() {
	// Clean up any existing data
	os.RemoveAll("./data")

	// Initialize BadgerDB
	opts := badger.DefaultOptions("./data/apm")
	opts.Logger = nil
	db, err := badger.Open(opts)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// Create index registry with APM schema
	registry := index.NewRegistry()
	registry.Register(entityType, &index.EntityTypeConfig{
		Indexes: []index.FieldIndex{
			{Name: "service", Type: index.FieldTypeString, Path: mustParsePath("service")},
			{Name: "endpoint", Type: index.FieldTypeString, Path: mustParsePath("endpoint")},
			{Name: "method", Type: index.FieldTypeString, Path: mustParsePath("method")},
			{Name: "status_code", Type: index.FieldTypeInt, Path: mustParsePath("status_code")},
			{Name: "latency_ms", Type: index.FieldTypeInt, Path: mustParsePath("latency_ms")},
			{Name: "tags", Type: index.FieldTypeStringArray, Path: mustParsePath("tags")},
		},
	})

	// Create entity store
	kv := badgerstore.NewKVStore(db)
	idx := badgerstore.NewIndexStore(db)
	es := store.NewEntityStore(kv, idx, registry, store.NewMsgpackSerializer())
	defer es.Close()

	if err := es.SyncIndexes(); err != nil {
		log.Fatalf("failed to sync indexes: %v", err)
	}

	fmt.Println("=== Chrono APM Example ===")
	fmt.Println()

	// Simulate request traffic over the past hour
	fmt.Println("Simulating 1000 HTTP requests over the past hour...")
	simulateTraffic(es, 1000, time.Hour)
	fmt.Println()

	// Run example queries
	runQueryExamples(es)
}

func simulateTraffic(es *store.EntityStore, count int, duration time.Duration) {
	now := time.Now()
	start := now.Add(-duration)

	for i := range count {
		// Random timestamp within the duration
		ts := start.Add(time.Duration(rand.Int63n(int64(duration))))

		// Pick random service and endpoint
		service := services[rand.Intn(len(services))]
		endpoint := endpoints[service][rand.Intn(len(endpoints[service]))]
		method := methods[rand.Intn(len(methods))]

		// Generate realistic latency (most fast, some slow)
		latency := generateLatency()

		// Generate status code (mostly 200s, some errors)
		statusCode := generateStatusCode()

		// Generate tags based on request characteristics
		tags := generateTags(latency, statusCode)

		req := &entity.Entity{
			ID:        fmt.Sprintf("req-%d", i),
			Type:      entityType,
			Timestamp: ts.UnixNano(),
			Fields: map[string]entity.Value{
				"service":     entity.NewString(service),
				"endpoint":    entity.NewString(endpoint),
				"method":      entity.NewString(method),
				"status_code": entity.NewInt(int64(statusCode)),
				"latency_ms":  entity.NewInt(int64(latency)),
				"tags":        entity.NewStringArray(tags),
			},
		}

		if err := es.Write(req); err != nil {
			log.Printf("failed to write request: %v", err)
		}
	}
}

func generateLatency() int {
	// 70% fast (10-100ms), 20% medium (100-500ms), 10% slow (500-3000ms)
	r := rand.Float64()
	switch {
	case r < 0.70:
		return 10 + rand.Intn(90)
	case r < 0.90:
		return 100 + rand.Intn(400)
	default:
		return 500 + rand.Intn(2500)
	}
}

func generateStatusCode() int {
	// 85% success, 10% client errors, 5% server errors
	r := rand.Float64()
	switch {
	case r < 0.85:
		return 200
	case r < 0.90:
		return 201
	case r < 0.95:
		codes := []int{400, 401, 403, 404}
		return codes[rand.Intn(len(codes))]
	default:
		codes := []int{500, 502, 503}
		return codes[rand.Intn(len(codes))]
	}
}

func generateTags(latency, statusCode int) []string {
	var tags []string

	if rand.Float64() < 0.6 {
		tags = append(tags, "authenticated")
	}
	if rand.Float64() < 0.3 {
		tags = append(tags, "cached")
	}
	if rand.Float64() < 0.1 {
		tags = append(tags, "rate-limited")
	}
	if latency > 500 {
		tags = append(tags, "slow")
	}
	if statusCode >= 500 && rand.Float64() < 0.5 {
		tags = append(tags, "retry")
	}
	if rand.Float64() < 0.2 {
		tags = append(tags, "external-call")
	}

	return tags
}

func runQueryExamples(es *store.EntityStore) {
	now := time.Now()
	oneHourAgo := now.Add(-time.Hour)

	// Query 1: All requests in the last hour (time-series)
	fmt.Println("--- Query 1: All requests in the last hour ---")
	results, err := es.Query(&store.Query{
		EntityType: entityType,
		TimeRange: &store.TimeRange{
			From: oneHourAgo.UnixNano(),
			To:   now.UnixNano(),
		},
	})
	if err != nil {
		log.Printf("query error: %v", err)
	} else {
		fmt.Printf("Total requests: %d\n", len(results))
	}
	fmt.Println()

	// Query 2: Slow requests (latency > 500ms)
	fmt.Println("--- Query 2: Slow requests (latency > 500ms) ---")
	results, err = es.Query(&store.Query{
		EntityType: entityType,
		Filters: []store.FieldFilter{
			{Field: "latency_ms", Op: store.OpGt, Value: entity.NewInt(500)},
		},
	})
	if err != nil {
		log.Printf("query error: %v", err)
	} else {
		fmt.Printf("Slow requests: %d\n", len(results))
		printSample(results, 3)
	}
	fmt.Println()

	// Query 3: Server errors (status_code >= 500)
	fmt.Println("--- Query 3: Server errors (status_code >= 500) ---")
	results, err = es.Query(&store.Query{
		EntityType: entityType,
		Filters: []store.FieldFilter{
			{Field: "status_code", Op: store.OpGte, Value: entity.NewInt(500)},
		},
	})
	if err != nil {
		log.Printf("query error: %v", err)
	} else {
		fmt.Printf("Server errors: %d\n", len(results))
		printSample(results, 3)
	}
	fmt.Println()

	// Query 4: Errors from payment-service (compound query)
	fmt.Println("--- Query 4: Errors from payment-service ---")
	results, err = es.Query(&store.Query{
		EntityType: entityType,
		Filters: []store.FieldFilter{
			{Field: "service", Op: store.OpEq, Value: entity.NewString("payment-service")},
			{Field: "status_code", Op: store.OpGte, Value: entity.NewInt(400)},
		},
	})
	if err != nil {
		log.Printf("query error: %v", err)
	} else {
		fmt.Printf("Payment service errors: %d\n", len(results))
		printSample(results, 3)
	}
	fmt.Println()

	// Query 5: Requests tagged as "slow" (array contains)
	fmt.Println("--- Query 5: Requests tagged as 'slow' ---")
	results, err = es.Query(&store.Query{
		EntityType: entityType,
		Filters: []store.FieldFilter{
			{Field: "tags", Op: store.OpContains, Value: entity.NewString("slow")},
		},
	})
	if err != nil {
		log.Printf("query error: %v", err)
	} else {
		fmt.Printf("Requests tagged 'slow': %d\n", len(results))
		printSample(results, 3)
	}
	fmt.Println()

	// Query 6: Cached requests that were still slow (compound with array)
	fmt.Println("--- Query 6: Cached requests that were still slow ---")
	results, err = es.Query(&store.Query{
		EntityType: entityType,
		Filters: []store.FieldFilter{
			{Field: "tags", Op: store.OpContains, Value: entity.NewString("cached")},
			{Field: "latency_ms", Op: store.OpGt, Value: entity.NewInt(200)},
		},
	})
	if err != nil {
		log.Printf("query error: %v", err)
	} else {
		fmt.Printf("Slow cached requests: %d\n", len(results))
		printSample(results, 3)
	}
	fmt.Println()

	// Query 7: Specific endpoint analysis
	fmt.Println("--- Query 7: POST requests to /orders/create ---")
	results, err = es.Query(&store.Query{
		EntityType: entityType,
		Filters: []store.FieldFilter{
			{Field: "endpoint", Op: store.OpEq, Value: entity.NewString("/orders/create")},
			{Field: "method", Op: store.OpEq, Value: entity.NewString("POST")},
		},
	})
	if err != nil {
		log.Printf("query error: %v", err)
	} else {
		fmt.Printf("POST /orders/create requests: %d\n", len(results))
		if len(results) > 0 {
			// Calculate average latency
			var totalLatency int64
			for _, r := range results {
				totalLatency += r.Fields["latency_ms"].I
			}
			fmt.Printf("Average latency: %dms\n", totalLatency/int64(len(results)))
		}
	}
	fmt.Println()

	// Query 8: Recent errors with limit
	fmt.Println("--- Query 8: Last 5 errors (with limit) ---")
	results, err = es.Query(&store.Query{
		EntityType: entityType,
		Filters: []store.FieldFilter{
			{Field: "status_code", Op: store.OpGte, Value: entity.NewInt(400)},
		},
		Limit: 5,
	})
	if err != nil {
		log.Printf("query error: %v", err)
	} else {
		fmt.Printf("Recent errors (limited to 5): %d\n", len(results))
		for _, r := range results {
			ts := time.Unix(0, r.Timestamp)
			fmt.Printf("  [%s] %s %s -> %d (%dms)\n",
				ts.Format("15:04:05"),
				r.Fields["method"].S,
				r.Fields["endpoint"].S,
				r.Fields["status_code"].I,
				r.Fields["latency_ms"].I,
			)
		}
	}
}

func printSample(results []*entity.Entity, n int) {
	if len(results) == 0 {
		return
	}
	fmt.Println("Sample:")
	for i, r := range results {
		if i >= n {
			break
		}
		ts := time.Unix(0, r.Timestamp)
		tags := extractTags(r.Fields["tags"])
		fmt.Printf("  [%s] %s %s %s -> %d (%dms) tags=%v\n",
			ts.Format("15:04:05"),
			r.Fields["service"].S,
			r.Fields["method"].S,
			r.Fields["endpoint"].S,
			r.Fields["status_code"].I,
			r.Fields["latency_ms"].I,
			tags,
		)
	}
}

func extractTags(v entity.Value) []string {
	if v.Kind != entity.KindArray {
		return nil
	}
	tags := make([]string, len(v.Arr))
	for i, t := range v.Arr {
		tags[i] = t.S
	}
	return tags
}

func mustParsePath(s string) entity.Path {
	p, err := entity.ParsePath(s)
	if err != nil {
		panic(err)
	}
	return p
}
