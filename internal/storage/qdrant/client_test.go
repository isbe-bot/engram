package qdrant

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPointIDDeterministicUUIDShape(t *testing.T) {
	a := PointID("mem-1")
	b := PointID("mem-1")
	if a != b {
		t.Fatalf("expected deterministic point id: %s vs %s", a, b)
	}
	if len(a) != 36 || a[8] != '-' || a[13] != '-' || a[18] != '-' || a[23] != '-' {
		t.Fatalf("expected UUID-shaped id, got %q", a)
	}
}

func TestClientEnsureCollectionAndUpsert(t *testing.T) {
	var sawCollection bool
	var sawUpsert bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/collections/engram_test":
			if r.Method != http.MethodPut {
				t.Fatalf("unexpected method for collection: %s", r.Method)
			}
			sawCollection = true
			writeQdrantOK(t, w, map[string]any{"status": "ok"})
		case "/collections/engram_test/points":
			if r.Method != http.MethodPut {
				t.Fatalf("unexpected method for upsert: %s", r.Method)
			}
			if r.URL.Query().Get("wait") != "true" {
				t.Fatalf("expected wait=true")
			}
			sawUpsert = true
			writeQdrantOK(t, w, map[string]any{"status": "ok"})
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := New(server.URL, "engram_test")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	if err := client.EnsureCollection(context.Background(), 16); err != nil {
		t.Fatalf("ensure collection: %v", err)
	}
	if err := client.Upsert(context.Background(), []Point{{ID: PointID("mem-1"), Vector: []float32{1, 0}, Payload: map[string]any{"object_id": "mem-1"}}}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if !sawCollection || !sawUpsert {
		t.Fatalf("expected collection and upsert calls")
	}
}

func TestClientSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/collections/engram_test/points/search" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		writeQdrantOK(t, w, map[string]any{
			"result": []map[string]any{{
				"id":      PointID("mem-1"),
				"score":   0.9,
				"payload": map[string]any{"object_id": "mem-1"},
			}},
		})
	}))
	defer server.Close()

	client, err := New(server.URL, "engram_test")
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	results, err := client.Search(context.Background(), []float32{1, 0}, 3, nil)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].Payload["object_id"] != "mem-1" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func writeQdrantOK(t *testing.T, w http.ResponseWriter, payload map[string]any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
