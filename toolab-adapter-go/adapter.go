// Package toolab provides a Toolab Adapter SDK that any Go HTTP application
// can mount to become toolab-ready.
//
// Usage with net/http:
//
//	adapter := toolab.NewAdapter(toolab.Config{
//	    AppName:    "my-app",
//	    AppVersion: "1.0.0",
//	})
//	http.Handle("/_toolab/", adapter.Handler())
//
// Usage with Gin:
//
//	adapter := toolab.NewAdapter(toolab.Config{...})
//	adapter.RegisterGin(router.Group("/_toolab"))
package toolab

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Config configures the adapter. Only AppName and AppVersion are required.
// All other fields enable optional capabilities.
type Config struct {
	AppName    string
	AppVersion string

	// DB enables state.fingerprint, state.snapshot, state.restore, state.reset.
	// If nil, state capabilities are not advertised.
	DB *sql.DB

	// StateProvider overrides the default DB-based state implementation.
	// If set, DB is ignored for state operations.
	StateProvider StateProvider

	// MetricsProvider enables the "metrics" capability.
	MetricsProvider MetricsProvider

	// LogsProvider enables the "logs" capability.
	LogsProvider LogsProvider

	// TracesProvider enables the "traces" capability.
	TracesProvider TracesProvider

	// SeedProvider enables the "seed" capability.
	SeedProvider SeedProvider
}

// Adapter is the toolab adapter instance.
type Adapter struct {
	cfg       Config
	mu        sync.RWMutex
	snapshots map[string]snapshotMeta
}

type snapshotMeta struct {
	ID          string
	Fingerprint string
	Label       string
	CreatedAt   time.Time
}

// NewAdapter creates a new adapter with the given configuration.
func NewAdapter(cfg Config) *Adapter {
	return &Adapter{
		cfg:       cfg,
		snapshots: make(map[string]snapshotMeta),
	}
}

// Handler returns an http.Handler that serves all adapter endpoints.
// Mount it at /_toolab/:
//
//	http.Handle("/_toolab/", http.StripPrefix("/_toolab", adapter.Handler()))
func (a *Adapter) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/manifest", a.handleManifest)

	if a.hasState() {
		mux.HandleFunc("/state/fingerprint", a.handleStateFingerprint)
		mux.HandleFunc("/state/snapshot", a.handleStateSnapshot)
		mux.HandleFunc("/state/restore", a.handleStateRestore)
		mux.HandleFunc("/state/reset", a.handleStateReset)
	}
	if a.cfg.SeedProvider != nil {
		mux.HandleFunc("/seed", a.handleSeed)
	}
	if a.cfg.MetricsProvider != nil {
		mux.HandleFunc("/metrics", a.handleMetrics)
	}
	if a.cfg.TracesProvider != nil {
		mux.HandleFunc("/traces", a.handleTraces)
	}
	if a.cfg.LogsProvider != nil {
		mux.HandleFunc("/logs", a.handleLogs)
	}
	return mux
}

func (a *Adapter) capabilities() []string {
	caps := make([]string, 0)
	if a.hasState() {
		caps = append(caps, "state.fingerprint", "state.snapshot", "state.restore", "state.reset")
	}
	if a.cfg.SeedProvider != nil {
		caps = append(caps, "seed")
	}
	if a.cfg.MetricsProvider != nil {
		caps = append(caps, "metrics")
	}
	if a.cfg.TracesProvider != nil {
		caps = append(caps, "traces")
	}
	if a.cfg.LogsProvider != nil {
		caps = append(caps, "logs")
	}
	return caps
}

func (a *Adapter) hasState() bool {
	return a.cfg.StateProvider != nil || a.cfg.DB != nil
}

func (a *Adapter) stateProvider() StateProvider {
	if a.cfg.StateProvider != nil {
		return a.cfg.StateProvider
	}
	if a.cfg.DB != nil {
		return &defaultDBState{db: a.cfg.DB}
	}
	return nil
}

// --- Handlers ---

func (a *Adapter) handleManifest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"adapter_version": "1",
		"app_name":        a.cfg.AppName,
		"app_version":     a.cfg.AppVersion,
		"capabilities":    a.capabilities(),
	})
}

func (a *Adapter) handleStateFingerprint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	sp := a.stateProvider()
	fp, err := sp.Fingerprint(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"fingerprint": fp,
		"scope":       "full",
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *Adapter) handleStateSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	var body struct {
		Label string `json:"label"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	sp := a.stateProvider()
	id, fp, err := sp.Snapshot(r.Context(), body.Label)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}

	now := time.Now().UTC()
	a.mu.Lock()
	a.snapshots[id] = snapshotMeta{ID: id, Fingerprint: fp, Label: body.Label, CreatedAt: now}
	a.mu.Unlock()

	writeJSON(w, http.StatusCreated, map[string]any{
		"snapshot_id": id,
		"fingerprint": fp,
		"label":       body.Label,
		"created_at":  now.Format(time.RFC3339),
	})
}

func (a *Adapter) handleStateRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	var body struct {
		SnapshotID string `json:"snapshot_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.SnapshotID == "" {
		writeError(w, http.StatusBadRequest, "seed_invalid", "snapshot_id is required")
		return
	}

	a.mu.RLock()
	meta, ok := a.snapshots[body.SnapshotID]
	a.mu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, "snapshot_not_found", "snapshot "+body.SnapshotID+" does not exist")
		return
	}

	sp := a.stateProvider()
	if err := sp.Restore(r.Context(), body.SnapshotID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"restored":    true,
		"snapshot_id": body.SnapshotID,
		"fingerprint": meta.Fingerprint,
	})
}

func (a *Adapter) handleStateReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}
	sp := a.stateProvider()
	if err := sp.Reset(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	fp, _ := sp.Fingerprint(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"reset":       true,
		"fingerprint": fp,
	})
}

func (a *Adapter) handleSeed(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var body struct {
			RunSeed string   `json:"run_seed"`
			Scope   []string `json:"scope"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RunSeed == "" {
			writeError(w, http.StatusBadRequest, "seed_invalid", "run_seed is required")
			return
		}
		result, err := a.cfg.SeedProvider.Apply(r.Context(), body.RunSeed, body.Scope)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"applied":       true,
			"run_seed":      body.RunSeed,
			"scope_applied": result.Applied,
			"scope_ignored": result.Ignored,
		})
	case http.MethodDelete:
		if err := a.cfg.SeedProvider.Clear(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"cleared": true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST or DELETE")
	}
}

func (a *Adapter) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	metrics, err := a.cfg.MetricsProvider.Snapshot(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"metrics":      metrics,
	})
}

func (a *Adapter) handleTraces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	since := parseSince(r.URL.Query().Get("since"))
	limit := parseInt(r.URL.Query().Get("limit"), 100)

	traces, err := a.cfg.TracesProvider.Collect(r.Context(), since, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"traces":       traces,
	})
}

func (a *Adapter) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
		return
	}
	since := parseSince(r.URL.Query().Get("since"))
	limit := parseInt(r.URL.Query().Get("limit"), 500)
	level := r.URL.Query().Get("level")

	lines, err := a.cfg.LogsProvider.Collect(r.Context(), since, limit, level)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"collected_at": time.Now().UTC().Format(time.RFC3339),
		"lines":        lines,
	})
}

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}

func parseSince(s string) time.Time {
	if s == "" {
		return time.Now().UTC().Add(-5 * time.Minute)
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Now().UTC().Add(-5 * time.Minute)
	}
	return t
}

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// stripPrefix is unused but reserved for potential future use with path routing.
var _ = strings.TrimPrefix
