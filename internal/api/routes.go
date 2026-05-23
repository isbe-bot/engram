package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/aileun/engram/internal/events"
)

type ingestor interface {
	Ingest(ctx context.Context, env events.Envelope) error
}

type searcher interface {
	Search(ctx context.Context, query string, limit int) ([]map[string]any, error)
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

	mux.HandleFunc("/v1/memory/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"error": "method not allowed"})
			return
		}
		if deps.Search == nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"error": "search service unavailable"})
			return
		}

		q := strings.TrimSpace(r.URL.Query().Get("q"))
		limit := 20
		if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
			if v, err := strconv.Atoi(raw); err == nil {
				limit = v
			}
		}

		results, err := deps.Search.Search(r.Context(), q, limit)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": results, "count": len(results)})
	})
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
