package discovery

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAPIFetcher_FetchURLGzipAndMaxBytes(t *testing.T) {
	specPath := filepath.Join("..", "..", "..", "testdata", "openapi", "petstore.yaml")
	specBytes, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read spec fixture: %v", err)
	}

	const token = "super-secret-token"
	if err := os.Setenv("OPENAPI_TEST_TOKEN", token); err != nil {
		t.Fatalf("set env: %v", err)
	}
	t.Cleanup(func() { _ = os.Unsetenv("OPENAPI_TEST_TOKEN") })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Encoding", "gzip")
		var buf bytes.Buffer
		gzw := gzip.NewWriter(&buf)
		_, _ = gzw.Write(specBytes)
		_ = gzw.Close()
		_, _ = w.Write(buf.Bytes())
	}))
	defer srv.Close()

	auth, err := ParseAuthFlag("bearer:OPENAPI_TEST_TOKEN")
	if err != nil {
		t.Fatalf("parse auth: %v", err)
	}
	fetcher := NewOpenAPIFetcher(HTTPConfig{MaxBytes: int64(len(specBytes) + 100)})
	doc, hash, info, warnings, err := fetcher.Fetch(context.Background(), srv.URL, auth)
	if err != nil {
		t.Fatalf("fetch openapi gzip: %v", err)
	}
	if doc == nil {
		t.Fatalf("expected parsed openapi doc")
	}
	if hash == "" || info.Hash == "" {
		t.Fatalf("expected non-empty hashes")
	}
	if info.Source != "url" {
		t.Fatalf("expected url source, got %s", info.Source)
	}
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %v", warnings)
	}

	tooSmall := NewOpenAPIFetcher(HTTPConfig{MaxBytes: 128})
	if _, _, _, _, err := tooSmall.Fetch(context.Background(), srv.URL, auth); err == nil {
		t.Fatalf("expected max bytes error for small limit")
	}
}

func TestToolabFetcher_ManifestAndProfileStableHashes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/manifest":
			_, _ = w.Write([]byte(`{"app_version":"1.0.0","capabilities":["profile"],"adapter_version":"1","app_name":"svc","standard_version":"1.1"}`))
		case "/profile":
			_, _ = w.Write([]byte(`{"profile_version":"1","standard_version":"1.1","suggested_flows":{"flows":[{"id":"flow-a","requests":[{"method":"GET","path":"/health"}]}]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := NewToolabFetcher(srv.URL, HTTPConfig{})
	manifestA, mHashA, _, err := f.Manifest(context.Background(), nil)
	if err != nil {
		t.Fatalf("manifest A: %v", err)
	}
	manifestB, mHashB, _, err := f.Manifest(context.Background(), nil)
	if err != nil {
		t.Fatalf("manifest B: %v", err)
	}
	if manifestA.AppName != "svc" || manifestB.AppName != "svc" {
		t.Fatalf("unexpected manifest app name")
	}
	if mHashA != mHashB {
		t.Fatalf("manifest hash must be stable")
	}

	profileA, pHashA, _, err := f.Profile(context.Background(), nil)
	if err != nil {
		t.Fatalf("profile A: %v", err)
	}
	profileB, pHashB, _, err := f.Profile(context.Background(), nil)
	if err != nil {
		t.Fatalf("profile B: %v", err)
	}
	if pHashA != pHashB {
		t.Fatalf("profile hash must be stable")
	}
	var flows map[string]any
	if err := json.Unmarshal(profileA.SuggestedFlows, &flows); err != nil {
		t.Fatalf("profile suggested_flows decode: %v", err)
	}
	if len(profileB.SuggestedFlows) == 0 {
		t.Fatalf("expected suggested_flows content")
	}
}
