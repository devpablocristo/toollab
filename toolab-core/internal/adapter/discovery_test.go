package adapter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDiscoverSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_toolab/manifest" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"adapter_version": "1",
			"app_name":        "test-app",
			"app_version":     "2.0.0",
			"capabilities":    []string{"state.fingerprint", "state.snapshot", "metrics"},
		})
	}))
	defer srv.Close()

	info := Discover(context.Background(), srv.URL)
	if info == nil {
		t.Fatal("expected adapter info, got nil")
	}
	if !info.Available {
		t.Error("expected Available=true")
	}
	if info.AppName != "test-app" {
		t.Errorf("expected app_name=test-app, got %s", info.AppName)
	}
	if !info.HasCapability("state.fingerprint") {
		t.Error("expected state.fingerprint capability")
	}
	if !info.HasCapability("metrics") {
		t.Error("expected metrics capability")
	}
	if info.HasCapability("seed") {
		t.Error("should not have seed capability")
	}
}

func TestDiscoverNoAdapter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	info := Discover(context.Background(), srv.URL)
	if info != nil {
		t.Errorf("expected nil for server without adapter, got %+v", info)
	}
}

func TestDiscoverTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	info := Discover(ctx, srv.URL)
	if info != nil {
		t.Errorf("expected nil for timeout, got %+v", info)
	}
}

func TestDiscoverBadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	info := Discover(context.Background(), srv.URL)
	if info != nil {
		t.Errorf("expected nil for bad JSON, got %+v", info)
	}
}

func TestClientStateFingerprint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/state/fingerprint" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"fingerprint": "sha256:abc123",
				"scope":       "full",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	fp, err := client.StateFingerprint(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if fp != "sha256:abc123" {
		t.Errorf("expected sha256:abc123, got %s", fp)
	}
}

func TestClientStateSnapshot(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/state/snapshot" && r.Method == http.MethodPost {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"snapshot_id": "snap_001",
				"fingerprint": "sha256:def456",
				"label":       body["label"],
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	result, err := client.StateSnapshot(context.Background(), "pre-run")
	if err != nil {
		t.Fatal(err)
	}
	if result.SnapshotID != "snap_001" {
		t.Errorf("expected snap_001, got %s", result.SnapshotID)
	}
	if result.Fingerprint != "sha256:def456" {
		t.Errorf("expected sha256:def456, got %s", result.Fingerprint)
	}
}

func TestClientMetrics(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" && r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"collected_at": "2025-01-01T00:00:00Z",
				"metrics": []map[string]any{
					{"name": "requests_total", "type": "counter", "value": 42},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	metrics, err := client.Metrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 1 {
		t.Fatalf("expected 1 metric, got %d", len(metrics))
	}
	if metrics[0]["name"] != "requests_total" {
		t.Errorf("expected requests_total, got %v", metrics[0]["name"])
	}
}

func TestClientSeedApplyAndClear(t *testing.T) {
	var appliedSeed string
	var cleared bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/seed" {
			switch r.Method {
			case http.MethodPost:
				var body map[string]any
				_ = json.NewDecoder(r.Body).Decode(&body)
				appliedSeed, _ = body["run_seed"].(string)
				_ = json.NewEncoder(w).Encode(map[string]any{"applied": true})
				return
			case http.MethodDelete:
				cleared = true
				_ = json.NewEncoder(w).Encode(map[string]any{"cleared": true})
				return
			}
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	client := NewClient(srv.URL)

	if err := client.SeedApply(context.Background(), "42", []string{"uuid"}); err != nil {
		t.Fatal(err)
	}
	if appliedSeed != "42" {
		t.Errorf("expected seed 42, got %s", appliedSeed)
	}

	if err := client.SeedClear(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !cleared {
		t.Error("expected seed cleared")
	}
}
