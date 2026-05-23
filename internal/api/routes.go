package api

import (
	"encoding/json"
	"net/http"
)

func registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "service": "engramd"})
	})

	mux.HandleFunc("/v1/events/ingest", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "accepted", "message": "TODO ingest pipeline"})
	})

	mux.HandleFunc("/v1/memory/search", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	})
}
