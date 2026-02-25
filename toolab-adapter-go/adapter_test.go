package toolab

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestManifestMinimal(t *testing.T) {
	a := NewAdapter(Config{AppName: "test-app", AppVersion: "1.0.0"})
	handler := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/manifest", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["adapter_version"] != "1" {
		t.Errorf("expected adapter_version 1, got %v", resp["adapter_version"])
	}
	if resp["app_name"] != "test-app" {
		t.Errorf("expected app_name test-app, got %v", resp["app_name"])
	}
	caps, ok := resp["capabilities"].([]any)
	if !ok {
		t.Fatalf("expected capabilities array, got %T", resp["capabilities"])
	}
	if len(caps) != 0 {
		t.Errorf("expected 0 capabilities for minimal config, got %d", len(caps))
	}
}

func TestManifestWithProviders(t *testing.T) {
	a := NewAdapter(Config{
		AppName:         "test-app",
		AppVersion:      "2.0.0",
		MetricsProvider: &mockMetrics{},
		LogsProvider:    &mockLogs{},
		SeedProvider:    &mockSeed{},
	})
	handler := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/manifest", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	caps := resp["capabilities"].([]any)

	expected := map[string]bool{"seed": false, "metrics": false, "logs": false}
	for _, c := range caps {
		expected[c.(string)] = true
	}
	for cap, found := range expected {
		if !found {
			t.Errorf("expected capability %q not found", cap)
		}
	}
}

func TestMetricsEndpoint(t *testing.T) {
	a := NewAdapter(Config{
		AppName:         "test-app",
		AppVersion:      "1.0.0",
		MetricsProvider: &mockMetrics{},
	})
	handler := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	metrics := resp["metrics"].([]any)
	if len(metrics) != 1 {
		t.Errorf("expected 1 metric, got %d", len(metrics))
	}
}

func TestLogsEndpoint(t *testing.T) {
	a := NewAdapter(Config{
		AppName:      "test-app",
		AppVersion:   "1.0.0",
		LogsProvider: &mockLogs{},
	})
	handler := a.Handler()

	req := httptest.NewRequest(http.MethodGet, "/logs?limit=10", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	lines := resp["lines"].([]any)
	if len(lines) != 1 {
		t.Errorf("expected 1 line, got %d", len(lines))
	}
}

func TestSeedApplyAndClear(t *testing.T) {
	a := NewAdapter(Config{
		AppName:      "test-app",
		AppVersion:   "1.0.0",
		SeedProvider: &mockSeed{},
	})
	handler := a.Handler()

	// Apply seed
	body := `{"run_seed":"42","scope":["uuid","timestamp"]}`
	req := httptest.NewRequest(http.MethodPost, "/seed", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["applied"] != true {
		t.Error("expected applied=true")
	}

	// Clear seed
	req = httptest.NewRequest(http.MethodDelete, "/seed", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	_ = json.NewDecoder(w.Body).Decode(&resp)
	if resp["cleared"] != true {
		t.Error("expected cleared=true")
	}
}

func TestMethodNotAllowed(t *testing.T) {
	a := NewAdapter(Config{AppName: "test-app", AppVersion: "1.0.0"})
	handler := a.Handler()

	req := httptest.NewRequest(http.MethodPost, "/manifest", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != 405 {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

// --- Mocks ---

type mockMetrics struct{}

func (m *mockMetrics) Snapshot(_ context.Context) ([]Metric, error) {
	return []Metric{
		{Name: "requests_total", Type: "counter", Value: 100, Labels: map[string]string{"method": "GET"}},
	}, nil
}

type mockLogs struct{}

func (m *mockLogs) Collect(_ context.Context, _ time.Time, _ int, _ string) ([]LogLine, error) {
	return []LogLine{
		{Timestamp: "2025-01-01T00:00:00Z", Level: "INFO", Message: "test"},
	}, nil
}

type mockSeed struct{}

func (m *mockSeed) Apply(_ context.Context, _ string, scope []string) (SeedResult, error) {
	return SeedResult{Applied: []string{"uuid"}, Ignored: []string{"timestamp"}}, nil
}

func (m *mockSeed) Clear(_ context.Context) error { return nil }
