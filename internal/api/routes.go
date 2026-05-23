package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/isbe-bot/engram/internal/events"
	"github.com/isbe-bot/engram/internal/models"
	"github.com/isbe-bot/engram/internal/quality"
	"github.com/isbe-bot/engram/internal/retrieve"
	"github.com/isbe-bot/engram/pkg/contracts"
)

type ingestor interface {
	Ingest(ctx context.Context, env events.Envelope) error
}

type searcher interface {
	Search(ctx context.Context, q retrieve.Query) (retrieve.Response, error)
}

type qualityReporter interface {
	Metrics(ctx context.Context) (quality.Snapshot, error)
	Report(ctx context.Context) (quality.Report, error)
}

type curator interface {
	Curate(ctx context.Context, req contracts.MemoryWriteRequest) (models.MemoryObject, error)
}

type governor interface {
	Get(ctx context.Context, objectID string) (models.MemoryObject, error)
	Correct(ctx context.Context, objectID string, req contracts.MemoryCorrectRequest) (models.MemoryObject, error)
	Deprecate(ctx context.Context, objectID string, req contracts.MemoryDeprecateRequest) (models.MemoryObject, error)
	History(ctx context.Context, objectID string, req contracts.MemoryHistoryRequest) ([]map[string]any, error)
}

type pinger interface {
	Ping(ctx context.Context) error
}

func registerRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}

		sqliteOK := deps.Health != nil && deps.Health.Ping(r.Context()) == nil
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"service": "engramd",
			"time":    time.Now().UTC().Format(time.RFC3339),
			"storage": map[string]any{"sqlite_ok": sqliteOK},
		})
	})

	requireAuth := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !authorized(r, deps.APIKey) {
				w.Header().Set("WWW-Authenticate", `Bearer realm="engram"`)
				writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "unauthorized"})
				return
			}
			next(w, r)
		}
	}

	mux.HandleFunc("/metrics", requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		if deps.Quality == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "quality service unavailable"})
			return
		}
		metrics, err := deps.Quality.Metrics(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "metrics unavailable"})
			return
		}
		writePrometheusMetrics(w, metrics)
	}))

	mux.HandleFunc("/v1/events/ingest", requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		if deps.Ingest == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "ingest service unavailable"})
			return
		}

		var env events.Envelope
		if err := decodeJSONBody(w, r, deps.MaxBodyBytes, &env); err != nil {
			writeJSON(w, statusForDecodeError(err), map[string]any{"error": messageForDecodeError(err)})
			return
		}
		if err := deps.Ingest.Ingest(r.Context(), env); err != nil {
			status := http.StatusBadRequest
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "already exists") {
				status = http.StatusConflict
			}
			writeJSON(w, status, map[string]any{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted", "event_id": env.EventID})
	}))

	mux.HandleFunc("/v1/memory/curate", requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		if deps.Curate == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "curate service unavailable"})
			return
		}

		var req contracts.MemoryWriteRequest
		if err := decodeJSONBody(w, r, deps.MaxBodyBytes, &req); err != nil {
			writeJSON(w, statusForDecodeError(err), map[string]any{"error": messageForDecodeError(err)})
			return
		}
		obj, err := deps.Curate.Curate(r.Context(), req)
		if err != nil {
			status := http.StatusBadRequest
			if strings.Contains(strings.ToLower(err.Error()), "already exists") {
				status = http.StatusConflict
			}
			writeJSON(w, status, map[string]any{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusAccepted, map[string]any{"status": "accepted", "memory": obj})
	}))

	mux.HandleFunc("/v1/memory/search", requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		if deps.Search == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "search service unavailable"})
			return
		}

		queryText := strings.TrimSpace(r.URL.Query().Get("q"))
		status := strings.TrimSpace(r.URL.Query().Get("status"))
		cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
		includeEvents := parseBoolQuery(strings.TrimSpace(r.URL.Query().Get("include_events")))

		limit := 20
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			if v, err := strconv.Atoi(raw); err == nil {
				limit = v
			}
		}
		minConfidence := 0.0
		if raw := strings.TrimSpace(r.URL.Query().Get("min_confidence")); raw != "" {
			if v, err := strconv.ParseFloat(raw, 64); err == nil {
				minConfidence = v
			}
		}

		resp, err := deps.Search.Search(r.Context(), retrieve.Query{
			Text:          queryText,
			Status:        status,
			MinConfidence: minConfidence,
			Limit:         limit,
			Cursor:        cursor,
			IncludeEvents: includeEvents,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "search failed"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"results":     resp.Results,
			"count":       len(resp.Results),
			"next_cursor": resp.NextCursor,
			"filters": map[string]any{
				"q":              queryText,
				"status":         status,
				"min_confidence": minConfidence,
				"limit":          limit,
				"cursor":         cursor,
				"include_events": includeEvents,
			},
		})
	}))

	mux.HandleFunc("/v1/quality/report", requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		if deps.Quality == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "quality service unavailable"})
			return
		}
		report, err := deps.Quality.Report(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "quality report unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"report": report})
	}))

	mux.HandleFunc("/v1/quality/metrics", requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		if deps.Quality == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "quality service unavailable"})
			return
		}
		metrics, err := deps.Quality.Metrics(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "quality metrics unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"metrics": metrics})
	}))

	mux.HandleFunc("/v1/memory/", requireAuth(func(w http.ResponseWriter, r *http.Request) {
		if deps.Govern == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "govern service unavailable"})
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/v1/memory/")
		parts := strings.Split(path, "/")
		if len(parts) == 1 && strings.TrimSpace(parts[0]) != "" {
			if r.Method != http.MethodGet {
				writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
				return
			}
			objectID := parts[0]
			obj, err := deps.Govern.Get(r.Context(), objectID)
			if err != nil {
				status := http.StatusBadRequest
				if strings.Contains(strings.ToLower(err.Error()), "not found") {
					status = http.StatusNotFound
				}
				writeJSON(w, status, map[string]any{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"memory": obj})
			return
		}
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
			return
		}
		objectID := parts[0]
		action := parts[1]

		switch action {
		case "history":
			if r.Method != http.MethodGet {
				writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
				return
			}
			limit := 50
			if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
				if v, err := strconv.Atoi(raw); err == nil {
					limit = v
				}
			}
			before := 0
			if raw := strings.TrimSpace(r.URL.Query().Get("before")); raw != "" {
				if v, err := strconv.Atoi(raw); err == nil {
					before = v
				}
			}
			action := strings.TrimSpace(r.URL.Query().Get("action"))
			events, err := deps.Govern.History(r.Context(), objectID, contracts.MemoryHistoryRequest{Action: action, Before: before, Limit: limit})
			if err != nil {
				status := http.StatusBadRequest
				if strings.Contains(strings.ToLower(err.Error()), "not found") {
					status = http.StatusNotFound
				}
				writeJSON(w, status, map[string]any{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"object_id": objectID,
				"count":     len(events),
				"events":    events,
				"filters":   map[string]any{"action": action, "before": before, "limit": limit},
			})
		case "correct":
			if r.Method != http.MethodPost {
				writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
				return
			}
			var req contracts.MemoryCorrectRequest
			if err := decodeJSONBody(w, r, deps.MaxBodyBytes, &req); err != nil {
				writeJSON(w, statusForDecodeError(err), map[string]any{"error": messageForDecodeError(err)})
				return
			}
			obj, err := deps.Govern.Correct(r.Context(), objectID, req)
			if err != nil {
				status := http.StatusBadRequest
				if strings.Contains(strings.ToLower(err.Error()), "not found") {
					status = http.StatusNotFound
				}
				writeJSON(w, status, map[string]any{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "memory": obj})
		case "deprecate":
			if r.Method != http.MethodPost {
				writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
				return
			}
			var req contracts.MemoryDeprecateRequest
			if err := decodeJSONBody(w, r, deps.MaxBodyBytes, &req); err != nil {
				writeJSON(w, statusForDecodeError(err), map[string]any{"error": messageForDecodeError(err)})
				return
			}
			obj, err := deps.Govern.Deprecate(r.Context(), objectID, req)
			if err != nil {
				status := http.StatusBadRequest
				if strings.Contains(strings.ToLower(err.Error()), "not found") {
					status = http.StatusNotFound
				}
				writeJSON(w, status, map[string]any{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "memory": obj})
		default:
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "not found"})
		}
	}))
}

func authorized(r *http.Request, apiKey string) bool {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return true
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return false
	}
	got := strings.TrimSpace(auth[len("Bearer "):])
	return subtle.ConstantTimeCompare([]byte(got), []byte(apiKey)) == 1
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, maxBytes int64, dst any) error {
	if maxBytes <= 0 {
		maxBytes = 1 << 20
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain a single JSON value")
	}
	return nil
}

func statusForDecodeError(err error) int {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusBadRequest
}

func messageForDecodeError(err error) string {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return "request body too large"
	}
	return "invalid JSON body"
}

func parseBoolQuery(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writePrometheusMetrics(w http.ResponseWriter, metrics quality.Snapshot) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = fmt.Fprintf(w, "# HELP engram_events_total Total ingested events.\n")
	_, _ = fmt.Fprintf(w, "# TYPE engram_events_total gauge\nengram_events_total %d\n", metrics.EventCount)
	_, _ = fmt.Fprintf(w, "# HELP engram_memory_objects_total Total memory objects.\n")
	_, _ = fmt.Fprintf(w, "# TYPE engram_memory_objects_total gauge\nengram_memory_objects_total %d\n", metrics.MemoryObjectCount)
	_, _ = fmt.Fprintf(w, "# HELP engram_corrections_total Total memory corrections.\n")
	_, _ = fmt.Fprintf(w, "# TYPE engram_corrections_total gauge\nengram_corrections_total %d\n", metrics.CorrectionCount)
	_, _ = fmt.Fprintf(w, "# HELP engram_deprecations_total Total memory deprecations.\n")
	_, _ = fmt.Fprintf(w, "# TYPE engram_deprecations_total gauge\nengram_deprecations_total %d\n", metrics.DeprecationCount)
	if metrics.IngestionLagSeconds != nil {
		_, _ = fmt.Fprintf(w, "# HELP engram_ingestion_lag_seconds Current ingestion lag in seconds.\n")
		_, _ = fmt.Fprintf(w, "# TYPE engram_ingestion_lag_seconds gauge\nengram_ingestion_lag_seconds %d\n", *metrics.IngestionLagSeconds)
	}
}
