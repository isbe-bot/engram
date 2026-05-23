package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/aileun/engram/internal/curate"
	"github.com/aileun/engram/internal/govern"
	"github.com/aileun/engram/internal/ingest"
	"github.com/aileun/engram/internal/quality"
	"github.com/aileun/engram/internal/retrieve"
	sqlitestore "github.com/aileun/engram/internal/storage/sqlite"
)

type apiTestServer struct {
	server *httptest.Server
	store  *sqlitestore.Store
}

func newAPITestServer(t *testing.T) *apiTestServer {
	t.Helper()

	store, err := sqlitestore.New(filepath.Join(t.TempDir(), "engram.sqlite"))
	if err != nil {
		t.Fatalf("new sqlite store: %v", err)
	}
	if err := store.ApplyMigrations(); err != nil {
		_ = store.Close()
		t.Fatalf("apply migrations: %v", err)
	}

	mux := http.NewServeMux()
	registerRoutes(mux, Dependencies{
		Ingest:  ingest.NewService(store),
		Curate:  curate.NewService(store),
		Govern:  govern.NewService(store),
		Search:  retrieve.NewService(store, store),
		Quality: quality.NewService(store),
		Health:  store,
	})

	ts := &apiTestServer{server: httptest.NewServer(mux), store: store}
	t.Cleanup(func() {
		ts.server.Close()
		if err := ts.store.Close(); err != nil {
			t.Fatalf("close sqlite store: %v", err)
		}
	})
	return ts
}

func TestMemoryAPIIntegrationLifecycle(t *testing.T) {
	ts := newAPITestServer(t)
	client := ts.server.Client()

	health := doJSON(t, client, http.MethodGet, ts.server.URL+"/v1/health", nil, http.StatusOK)
	if health["service"] != "engramd" {
		t.Fatalf("unexpected health response: %+v", health)
	}

	ingestPayload := map[string]any{
		"event_id":       "evt-api-1",
		"event_type":     "task.completed",
		"environment_id": "api-test",
		"occurred_at":    "2026-05-23T05:00:00Z",
		"data":           map[string]any{"title": "API lifecycle smoke"},
	}
	ingestResp := doJSON(t, client, http.MethodPost, ts.server.URL+"/v1/events/ingest", ingestPayload, http.StatusAccepted)
	if ingestResp["event_id"] != "evt-api-1" {
		t.Fatalf("unexpected ingest response: %+v", ingestResp)
	}
	doJSON(t, client, http.MethodPost, ts.server.URL+"/v1/events/ingest", ingestPayload, http.StatusConflict)

	curatePayload := map[string]any{
		"object_id":      "mem-api-1",
		"type":           "decision",
		"content":        "ENGRAM API supports integration tests",
		"source_refs":    []string{"spec:api-integration"},
		"confidence":     0.85,
		"classification": "product",
		"envelope": map[string]any{
			"actor_id":    "api-test",
			"mutation_id": "mut-api-1",
			"signature":   "sig-api-1",
		},
	}
	curateResp := doJSON(t, client, http.MethodPost, ts.server.URL+"/v1/memory/curate", curatePayload, http.StatusAccepted)
	memory := mapValue(t, curateResp, "memory")
	if memory["object_id"] != "mem-api-1" {
		t.Fatalf("unexpected curated memory: %+v", memory)
	}

	correctPayload := map[string]any{
		"content":     "ENGRAM API supports HTTP lifecycle integration tests",
		"reason":      "Clarify API integration coverage",
		"source_refs": []string{"spec:api-integration"},
		"envelope": map[string]any{
			"actor_id":    "api-test",
			"mutation_id": "mut-api-2",
			"signature":   "sig-api-2",
		},
	}
	correctResp := doJSON(t, client, http.MethodPost, ts.server.URL+"/v1/memory/mem-api-1/correct", correctPayload, http.StatusOK)
	corrected := mapValue(t, correctResp, "memory")
	if corrected["content"] != "ENGRAM API supports HTTP lifecycle integration tests" {
		t.Fatalf("unexpected corrected memory: %+v", corrected)
	}

	getResp := doJSON(t, client, http.MethodGet, ts.server.URL+"/v1/memory/mem-api-1", nil, http.StatusOK)
	fetched := mapValue(t, getResp, "memory")
	if fetched["object_id"] != "mem-api-1" {
		t.Fatalf("unexpected fetched memory: %+v", fetched)
	}
	doJSON(t, client, http.MethodGet, ts.server.URL+"/v1/memory/missing-object", nil, http.StatusNotFound)

	searchResp := doJSON(t, client, http.MethodGet, ts.server.URL+"/v1/memory/search?q=HTTP&status=accepted&min_confidence=0.8&include_events=true", nil, http.StatusOK)
	if got := int(searchResp["count"].(float64)); got < 1 {
		t.Fatalf("expected at least one search result, got %+v", searchResp)
	}

	historyResp := doJSON(t, client, http.MethodGet, ts.server.URL+"/v1/memory/mem-api-1/history?action=corrected", nil, http.StatusOK)
	if got := int(historyResp["count"].(float64)); got != 1 {
		t.Fatalf("expected one corrected history event, got %+v", historyResp)
	}

	metricsResp := doJSON(t, client, http.MethodGet, ts.server.URL+"/v1/quality/metrics", nil, http.StatusOK)
	metrics := mapValue(t, metricsResp, "metrics")
	if got := int(metrics["event_count"].(float64)); got != 1 {
		t.Fatalf("expected event_count=1, got %+v", metrics)
	}
	if got := int(metrics["memory_object_count"].(float64)); got != 1 {
		t.Fatalf("expected memory_object_count=1, got %+v", metrics)
	}
	if got := int(metrics["correction_count"].(float64)); got != 1 {
		t.Fatalf("expected correction_count=1, got %+v", metrics)
	}
}

func TestMemoryAPIIntegrationValidation(t *testing.T) {
	ts := newAPITestServer(t)
	client := ts.server.Client()

	doRaw(t, client, http.MethodPost, ts.server.URL+"/v1/events/ingest", []byte(`{"event_id":`), http.StatusBadRequest)
	doJSON(t, client, http.MethodGet, ts.server.URL+"/v1/events/ingest", nil, http.StatusMethodNotAllowed)

	badCurate := map[string]any{
		"type":           "decision",
		"content":        "Missing approved source refs",
		"source_refs":    []string{"bad-prefix:123"},
		"classification": "product",
	}
	doJSON(t, client, http.MethodPost, ts.server.URL+"/v1/memory/curate", badCurate, http.StatusBadRequest)

	badCorrection := map[string]any{
		"content": "Will fail because envelope is missing",
		"reason":  "Envelope is intentionally absent",
	}
	doJSON(t, client, http.MethodPost, ts.server.URL+"/v1/memory/missing/correct", badCorrection, http.StatusBadRequest)
}

func doJSON(t *testing.T, client *http.Client, method, url string, body any, wantStatus int) map[string]any {
	t.Helper()

	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
	}
	return doRaw(t, client, method, url, raw, wantStatus)
}

func doRaw(t *testing.T, client *http.Client, method, url string, body []byte, wantStatus int) map[string]any {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Fatalf("close response body: %v", err)
		}
	}()

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if resp.StatusCode != wantStatus {
		t.Fatalf("unexpected status for %s %s: got=%d want=%d payload=%+v", method, url, resp.StatusCode, wantStatus, payload)
	}
	return payload
}

func mapValue(t *testing.T, m map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := m[key].(map[string]any)
	if !ok {
		t.Fatalf("expected %q to be an object, got %T in %+v", key, m[key], m)
	}
	return value
}
