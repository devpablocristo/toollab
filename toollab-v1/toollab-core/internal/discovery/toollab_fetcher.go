package discovery

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ToollabFetcher struct {
	BaseURL    string
	HTTPConfig HTTPConfig
	Client     *http.Client
}

func NewToollabFetcher(base string, cfg HTTPConfig) *ToollabFetcher {
	if cfg.TimeoutMS <= 0 {
		cfg.TimeoutMS = defaultTimeoutMS
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = defaultMaxBytes
	}
	return &ToollabFetcher{
		BaseURL:    strings.TrimRight(base, "/"),
		HTTPConfig: cfg,
		Client: &http.Client{
			Timeout: time.Duration(cfg.TimeoutMS) * time.Millisecond,
		},
	}
}

func (f *ToollabFetcher) Manifest(ctx context.Context, auth *AuthConfig) (*Manifest, string, []string, error) {
	var out Manifest
	hash, warnings, err := f.getJSON(ctx, "/manifest", auth, &out)
	if err != nil {
		return nil, "", warnings, err
	}
	return &out, hash, warnings, nil
}

func (f *ToollabFetcher) Profile(ctx context.Context, auth *AuthConfig) (*Profile, string, []string, error) {
	var out Profile
	hash, warnings, err := f.getJSON(ctx, "/profile", auth, &out)
	if err != nil {
		return nil, "", warnings, err
	}
	return &out, hash, warnings, nil
}

func (f *ToollabFetcher) OpenAPI(ctx context.Context, auth *AuthConfig) ([]byte, string, []string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.BaseURL+"/openapi", nil)
	if err != nil {
		return nil, "", nil, err
	}
	req.Header.Set("Accept-Encoding", "gzip")
	applyAuth(req, auth)

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, "", nil, fmt.Errorf("openapi status %d", resp.StatusCode)
	}
	raw, err := readLimitedBody(resp, f.HTTPConfig.MaxBytes)
	if err != nil {
		return nil, "", nil, err
	}
	return raw, HashBytes(raw), nil, nil
}

func (f *ToollabFetcher) Description(ctx context.Context, auth *AuthConfig) (*ServiceDescription, string, []string, error) {
	var out ServiceDescription
	hash, warnings, err := f.getJSON(ctx, "/description", auth, &out)
	if err != nil {
		return nil, "", warnings, err
	}
	return &out, hash, warnings, nil
}

func (f *ToollabFetcher) RawJSON(ctx context.Context, suffix string, auth *AuthConfig) (json.RawMessage, string, []string, error) {
	var decoded any
	hash, warnings, err := f.getJSON(ctx, suffix, auth, &decoded)
	if err != nil {
		return nil, "", warnings, err
	}
	raw, err := json.Marshal(decoded)
	if err != nil {
		return nil, "", warnings, err
	}
	return raw, hash, warnings, nil
}

func (f *ToollabFetcher) getJSON(ctx context.Context, suffix string, auth *AuthConfig, out any) (string, []string, error) {
	warnings := []string{}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.BaseURL+suffix, nil)
	if err != nil {
		return "", warnings, err
	}
	req.Header.Set("Accept-Encoding", "gzip")
	applyAuth(req, auth)
	resp, err := f.Client.Do(req)
	if err != nil {
		return "", warnings, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", warnings, fmt.Errorf("%s status %d", suffix, resp.StatusCode)
	}
	raw, err := readLimitedBody(resp, f.HTTPConfig.MaxBytes)
	if err != nil {
		return "", warnings, err
	}
	hash, canonical, err := JSONHashCanonical(raw)
	if err != nil {
		return "", warnings, fmt.Errorf("decode %s json: %w", suffix, err)
	}
	if err := json.Unmarshal(canonical, out); err != nil {
		return "", warnings, err
	}
	return hash, warnings, nil
}

func readLimitedBody(resp *http.Response, maxBytes int64) ([]byte, error) {
	var reader io.Reader = io.LimitReader(resp.Body, maxBytes+1)
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gzr, err := gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("decode gzip response: %w", err)
		}
		defer gzr.Close()
		reader = io.LimitReader(gzr, maxBytes+1)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maxBytes {
		return nil, fmt.Errorf("payload exceeds max bytes")
	}
	return raw, nil
}
