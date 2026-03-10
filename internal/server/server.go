package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cbodonnell/chrono/pkg/entity"
	"github.com/cbodonnell/chrono/pkg/store"
)

// Config holds server configuration.
type Config struct {
	Addr string
}

// Server is an embeddable HTTP server for chrono.
type Server struct {
	httpServer *http.Server
	store      *store.EntityStore
}

// New creates a new chrono server.
func New(es *store.EntityStore, cfg Config) *Server {
	s := &Server{
		store: es,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/entities", s.handleEntities)
	mux.HandleFunc("/entities/", s.handleEntity)
	mux.HandleFunc("/query", s.handleQuery)
	mux.HandleFunc("/health", s.handleHealth)

	s.httpServer = &http.Server{
		Addr:              cfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return s
}

// Start begins serving HTTP requests (blocks until shutdown).
func (s *Server) Start() error {
	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// handleHealth returns a simple health check response.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleEntities handles POST /entities for creating entities.
func (s *Server) handleEntities(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.createEntity(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleEntity handles GET/DELETE /entities/{type}/{id} and GET /entities/{type}/{id}/history.
func (s *Server) handleEntity(w http.ResponseWriter, r *http.Request) {
	// Parse path: /entities/{type}/{id} or /entities/{type}/{id}/history
	path := strings.TrimPrefix(r.URL.Path, "/entities/")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		http.Error(w, "invalid path: expected /entities/{type}/{id}", http.StatusBadRequest)
		return
	}

	entityType := parts[0]
	entityID := parts[1]

	// Check if this is a history request
	if len(parts) == 3 && parts[2] == "history" {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.getEntityHistory(w, r, entityType, entityID)
		return
	}

	// Regular entity request
	if len(parts) > 2 {
		http.Error(w, "invalid path: expected /entities/{type}/{id}", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getEntity(w, entityType, entityID)
	case http.MethodDelete:
		s.deleteEntity(w, entityType, entityID)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// createEntity handles POST /entities.
func (s *Server) createEntity(w http.ResponseWriter, r *http.Request) {
	var req EntityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	e, err := req.ToEntity()
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid entity: %v", err), http.StatusBadRequest)
		return
	}

	if err := s.store.Write(e); err != nil {
		http.Error(w, fmt.Sprintf("failed to write entity: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(EntityResponse{}.FromEntity(e))
}

// getEntity handles GET /entities/{type}/{id}.
func (s *Server) getEntity(w http.ResponseWriter, entityType, entityID string) {
	e, err := s.store.Get(entityType, entityID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "entity not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("failed to get entity: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(EntityResponse{}.FromEntity(e))
}

// deleteEntity handles DELETE /entities/{type}/{id}.
// Deletes ALL versions of the entity.
func (s *Server) deleteEntity(w http.ResponseWriter, entityType, entityID string) {
	if err := s.store.DeleteEntity(entityType, entityID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			http.Error(w, "entity not found", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("failed to delete entity: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HistoryResponse is the paginated response for entity history.
type HistoryResponse struct {
	Items      []EntityResponse `json:"items"`
	NextCursor string           `json:"next_cursor,omitempty"`
	HasMore    bool             `json:"has_more"`
}

// getEntityHistory handles GET /entities/{type}/{id}/history.
func (s *Server) getEntityHistory(w http.ResponseWriter, r *http.Request, entityType, entityID string) {
	opts := &store.HistoryOptions{}

	// Parse query parameters
	query := r.URL.Query()

	if fromStr := query.Get("from"); fromStr != "" {
		from, err := strconv.ParseInt(fromStr, 10, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid 'from' parameter: %v", err), http.StatusBadRequest)
			return
		}
		if opts.TimeRange == nil {
			opts.TimeRange = &store.TimeRange{}
		}
		opts.TimeRange.From = from
	}

	if toStr := query.Get("to"); toStr != "" {
		to, err := strconv.ParseInt(toStr, 10, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid 'to' parameter: %v", err), http.StatusBadRequest)
			return
		}
		if opts.TimeRange == nil {
			opts.TimeRange = &store.TimeRange{}
		}
		opts.TimeRange.To = to
	}

	// Handle cursor-based pagination
	if cursorStr := query.Get("cursor"); cursorStr != "" {
		cursor, err := strconv.ParseInt(cursorStr, 10, 64)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid 'cursor' parameter: %v", err), http.StatusBadRequest)
			return
		}
		if opts.TimeRange == nil {
			opts.TimeRange = &store.TimeRange{}
		}
		// Cursor represents the last timestamp seen
		// For forward order: start after the cursor
		// For reverse order: end before the cursor
		if query.Get("reverse") == "true" {
			opts.TimeRange.To = cursor - 1
		} else {
			opts.TimeRange.From = cursor + 1
		}
	}

	// Default limit for pagination if not specified
	requestedLimit := 0
	if limitStr := query.Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid 'limit' parameter: %v", err), http.StatusBadRequest)
			return
		}
		requestedLimit = limit
	} else {
		// Default limit when not specified
		requestedLimit = 100
	}

	// Request one extra to determine if there are more results
	opts.Limit = requestedLimit + 1

	if reverseStr := query.Get("reverse"); reverseStr == "true" {
		opts.Reverse = true
	}

	history, err := s.store.GetHistory(entityType, entityID, opts)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get history: %v", err), http.StatusInternalServerError)
		return
	}

	// Determine if there are more results
	hasMore := len(history) > requestedLimit
	if hasMore {
		history = history[:requestedLimit]
	}

	items := make([]EntityResponse, len(history))
	for i, e := range history {
		items[i] = EntityResponse{}.FromEntity(e)
	}

	// Build next cursor from the last item's timestamp
	var nextCursor string
	if hasMore && len(history) > 0 {
		lastItem := history[len(history)-1]
		nextCursor = strconv.FormatInt(lastItem.Timestamp, 10)
	}

	resp := HistoryResponse{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleQuery handles POST /query.
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	q, err := req.ToQuery()
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid query: %v", err), http.StatusBadRequest)
		return
	}

	results, err := s.store.Query(q)
	if err != nil {
		http.Error(w, fmt.Sprintf("query failed: %v", err), http.StatusInternalServerError)
		return
	}

	resp := make([]EntityResponse, len(results))
	for i, e := range results {
		resp[i] = EntityResponse{}.FromEntity(e)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// EntityRequest is the JSON representation for creating an entity.
type EntityRequest struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Timestamp int64                  `json:"timestamp,omitempty"`
	Fields    map[string]interface{} `json:"fields"`
}

// ToEntity converts the request to an entity.
func (r *EntityRequest) ToEntity() (*entity.Entity, error) {
	if r.ID == "" {
		return nil, errors.New("id is required")
	}
	if r.Type == "" {
		return nil, errors.New("type is required")
	}

	ts := r.Timestamp
	if ts == 0 {
		ts = time.Now().UnixNano()
	}

	fields := make(map[string]entity.Value)
	for k, v := range r.Fields {
		val, err := toValue(v)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", k, err)
		}
		fields[k] = val
	}

	return &entity.Entity{
		ID:        r.ID,
		Type:      r.Type,
		Timestamp: ts,
		Fields:    fields,
	}, nil
}

// toValue converts a JSON value to an entity.Value.
func toValue(v interface{}) (entity.Value, error) {
	switch val := v.(type) {
	case float64:
		// JSON numbers are float64; check if it's an integer
		if val == float64(int64(val)) {
			return entity.NewInt(int64(val)), nil
		}
		return entity.NewFloat(val), nil
	case bool:
		return entity.NewBool(val), nil
	case string:
		return entity.NewString(val), nil
	case []interface{}:
		arr := make([]entity.Value, len(val))
		for i, elem := range val {
			elemVal, err := toValue(elem)
			if err != nil {
				return entity.Value{}, err
			}
			arr[i] = elemVal
		}
		return entity.NewArray(arr), nil
	case map[string]interface{}:
		obj := make(map[string]entity.Value, len(val))
		for k, v := range val {
			vv, err := toValue(v)
			if err != nil {
				return entity.Value{}, err
			}
			obj[k] = vv
		}
		return entity.NewObject(obj), nil
	case nil:
		return entity.Value{}, errors.New("null values not supported")
	default:
		return entity.Value{}, fmt.Errorf("unsupported type: %T", v)
	}
}

// EntityResponse is the JSON representation of an entity in responses.
type EntityResponse struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	Fields    map[string]interface{} `json:"fields"`
}

// FromEntity converts an entity to a response.
func (EntityResponse) FromEntity(e *entity.Entity) EntityResponse {
	fields := make(map[string]interface{})
	for k, v := range e.Fields {
		fields[k] = fromValue(v)
	}
	return EntityResponse{
		ID:        e.ID,
		Type:      e.Type,
		Timestamp: e.Timestamp,
		Fields:    fields,
	}
}

// fromValue converts an entity.Value to a JSON-compatible value.
func fromValue(v entity.Value) interface{} {
	switch v.Kind {
	case entity.KindInt:
		return v.I
	case entity.KindFloat:
		return v.F
	case entity.KindBool:
		return v.B
	case entity.KindString:
		return v.S
	case entity.KindArray:
		arr := make([]interface{}, len(v.Arr))
		for i, elem := range v.Arr {
			arr[i] = fromValue(elem)
		}
		return arr
	case entity.KindObject:
		obj := make(map[string]interface{}, len(v.Obj))
		for k, elem := range v.Obj {
			obj[k] = fromValue(elem)
		}
		return obj
	default:
		return nil
	}
}

// QueryRequest is the JSON representation of a query.
type QueryRequest struct {
	EntityType     string         `json:"entity_type"`
	Filters        []FilterSpec   `json:"filters,omitempty"`
	TimeRange      *TimeRangeSpec `json:"time_range,omitempty"`
	Limit          int            `json:"limit,omitempty"`
	Reverse        bool           `json:"reverse,omitempty"`
	IncludeHistory bool           `json:"include_history,omitempty"`
}

// FilterSpec is a single filter in a query.
type FilterSpec struct {
	Field string      `json:"field"`
	Op    string      `json:"op"`
	Value interface{} `json:"value"`
}

// TimeRangeSpec specifies a time range.
type TimeRangeSpec struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

// ToQuery converts the request to a store.Query.
func (r *QueryRequest) ToQuery() (*store.Query, error) {
	if r.EntityType == "" {
		return nil, errors.New("entity_type is required")
	}

	filters := make([]store.FieldFilter, len(r.Filters))
	for i, f := range r.Filters {
		op, err := parseOp(f.Op)
		if err != nil {
			return nil, err
		}
		val, err := toValue(f.Value)
		if err != nil {
			return nil, fmt.Errorf("filter %d value: %w", i, err)
		}
		filters[i] = store.FieldFilter{
			Field: f.Field,
			Op:    op,
			Value: val,
		}
	}

	var timeRange *store.TimeRange
	if r.TimeRange != nil {
		timeRange = &store.TimeRange{
			From: r.TimeRange.From,
			To:   r.TimeRange.To,
		}
	}

	return &store.Query{
		EntityType:     r.EntityType,
		Filters:        filters,
		TimeRange:      timeRange,
		Limit:          r.Limit,
		Reverse:        r.Reverse,
		IncludeHistory: r.IncludeHistory,
	}, nil
}

// parseOp converts a string operation to store.Op.
func parseOp(s string) (store.Op, error) {
	switch s {
	case "eq", "=", "==":
		return store.OpEq, nil
	case "lt", "<":
		return store.OpLt, nil
	case "lte", "<=":
		return store.OpLte, nil
	case "gt", ">":
		return store.OpGt, nil
	case "gte", ">=":
		return store.OpGte, nil
	case "contains":
		return store.OpContains, nil
	default:
		return 0, fmt.Errorf("unknown operation: %s", s)
	}
}
