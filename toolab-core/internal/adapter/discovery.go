// Package adapter provides a client for the Toolab Adapter spec v1.
// It probes a target's /_toolab/manifest endpoint and exposes typed
// methods for every adapter capability (state, seed, metrics, logs, traces).
package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Info holds the parsed manifest from a toolab-ready target.
type Info struct {
	Available      bool
	AdapterVersion string   `json:"adapter_version"`
	AppName        string   `json:"app_name"`
	AppVersion     string   `json:"app_version"`
	Capabilities   []string `json:"capabilities"`
	BaseURL        string   // /_toolab base URL
}

// HasCapability returns true if the adapter advertises the given capability.
func (i *Info) HasCapability(cap string) bool {
	for _, c := range i.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// Discover probes targetBaseURL + /_toolab/manifest and returns adapter info.
// Returns nil (no error) when the target simply doesn't have an adapter.
func Discover(ctx context.Context, targetBaseURL string) *Info {
	base := strings.TrimRight(targetBaseURL, "/") + "/_toolab"
	manifestURL := base + "/manifest"

	reqCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, manifestURL, nil)
	if err != nil {
		return nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var info Info
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil
	}
	if info.AdapterVersion == "" {
		return nil
	}

	info.Available = true
	info.BaseURL = base
	return &info
}

// Client talks to a toolab adapter over HTTP.
type Client struct {
	baseURL string // e.g. http://localhost:8080/_toolab
	http    *http.Client
}

// NewClient creates a client for the given adapter base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// --- State ---

// StateFingerprint returns the current data-state fingerprint.
func (c *Client) StateFingerprint(ctx context.Context) (string, error) {
	var resp struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := c.getJSON(ctx, "/state/fingerprint", &resp); err != nil {
		return "", err
	}
	return resp.Fingerprint, nil
}

// SnapshotResult holds the response from a state snapshot.
type SnapshotResult struct {
	SnapshotID  string `json:"snapshot_id"`
	Fingerprint string `json:"fingerprint"`
}

// StateSnapshot captures current state, returns snapshot ID + fingerprint.
func (c *Client) StateSnapshot(ctx context.Context, label string) (*SnapshotResult, error) {
	body := map[string]string{"label": label}
	var resp SnapshotResult
	if err := c.postJSON(ctx, "/state/snapshot", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// StateRestore restores to a previous snapshot.
func (c *Client) StateRestore(ctx context.Context, snapshotID string) error {
	body := map[string]string{"snapshot_id": snapshotID}
	var resp map[string]any
	return c.postJSON(ctx, "/state/restore", body, &resp)
}

// StateReset restores to initial/seed state.
func (c *Client) StateReset(ctx context.Context) error {
	var resp map[string]any
	return c.postJSON(ctx, "/state/reset", nil, &resp)
}

// --- Seed ---

// SeedApply puts the target into deterministic mode.
func (c *Client) SeedApply(ctx context.Context, seed string, scope []string) error {
	body := map[string]any{"run_seed": seed, "scope": scope}
	var resp map[string]any
	return c.postJSON(ctx, "/seed", body, &resp)
}

// SeedClear exits deterministic mode.
func (c *Client) SeedClear(ctx context.Context) error {
	return c.deleteJSON(ctx, "/seed")
}

// --- Observability ---

// Metrics returns the adapter's structured metrics snapshot.
func (c *Client) Metrics(ctx context.Context) ([]map[string]any, error) {
	var resp struct {
		Metrics []map[string]any `json:"metrics"`
	}
	if err := c.getJSON(ctx, "/metrics", &resp); err != nil {
		return nil, err
	}
	return resp.Metrics, nil
}

// Logs returns structured log lines from the adapter.
func (c *Client) Logs(ctx context.Context, since time.Time, limit int) ([]map[string]any, error) {
	path := fmt.Sprintf("/logs?since=%s&limit=%d", since.UTC().Format(time.RFC3339), limit)
	var resp struct {
		Lines []map[string]any `json:"lines"`
	}
	if err := c.getJSON(ctx, path, &resp); err != nil {
		return nil, err
	}
	return resp.Lines, nil
}

// Traces returns trace spans from the adapter.
func (c *Client) Traces(ctx context.Context, since time.Time, limit int) ([]map[string]any, error) {
	path := fmt.Sprintf("/traces?since=%s&limit=%d", since.UTC().Format(time.RFC3339), limit)
	var resp struct {
		Traces []map[string]any `json:"traces"`
	}
	if err := c.getJSON(ctx, path, &resp); err != nil {
		return nil, err
	}
	return resp.Traces, nil
}

// --- HTTP helpers ---

func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.doJSON(req, out)
}

func (c *Client) postJSON(ctx context.Context, path string, body any, out any) error {
	var bodyReader *strings.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = strings.NewReader(string(raw))
	} else {
		bodyReader = strings.NewReader("{}")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.doJSON(req, out)
}

func (c *Client) deleteJSON(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("adapter DELETE %s: status %d", path, resp.StatusCode)
	}
	return nil
}

func (c *Client) doJSON(req *http.Request, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("adapter %s %s: status %d", req.Method, req.URL.Path, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
