package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aileun/engram/internal/events"
	"github.com/aileun/engram/internal/models"
	"github.com/aileun/engram/internal/retrieve"
	"github.com/aileun/engram/pkg/contracts"
)

type ingestor interface {
	Ingest(ctx context.Context, env events.Envelope) error
}

type searcher interface {
	Search(ctx context.Context, q retrieve.Query) (retrieve.Response, error)
}

type curator interface {
	Curate(ctx context.Context, req contracts.MemoryWriteRequest) (models.MemoryObject, error)
}

type governor interface {
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

	mux.HandleFunc("/v1/events/ingest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		if deps.Ingest == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "ingest service unavailable"})
			return
		}

		var env events.Envelope
		if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
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
	})

	mux.HandleFunc("/v1/memory/curate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		if deps.Curate == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "curate service unavailable"})
			return
		}

		var req contracts.MemoryWriteRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
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
	})

	mux.HandleFunc("/v1/memory/search", func(w http.ResponseWriter, r *http.Request) {
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
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
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
	})

	mux.HandleFunc("/v1/memory/", func(w http.ResponseWriter, r *http.Request) {
		if deps.Govern == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "govern service unavailable"})
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/v1/memory/")
		parts := strings.Split(path, "/")
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
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
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
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON body"})
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
	})
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
