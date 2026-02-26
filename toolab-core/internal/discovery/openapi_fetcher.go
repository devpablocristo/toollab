package discovery

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"toolab-core/internal/gen"
	"toolab-core/pkg/utils"
)

const (
	defaultTimeoutMS = 10000
	defaultMaxBytes  = 20 << 20
)

type OpenAPIFetcher struct {
	HTTPConfig HTTPConfig
	Client     *http.Client
}

func NewOpenAPIFetcher(cfg HTTPConfig) *OpenAPIFetcher {
	if cfg.TimeoutMS <= 0 {
		cfg.TimeoutMS = defaultTimeoutMS
	}
	if cfg.MaxBytes <= 0 {
		cfg.MaxBytes = defaultMaxBytes
	}
	return &OpenAPIFetcher{
		HTTPConfig: cfg,
		Client: &http.Client{
			Timeout: time.Duration(cfg.TimeoutMS) * time.Millisecond,
		},
	}
}

func (f *OpenAPIFetcher) Fetch(ctx context.Context, input string, auth *AuthConfig) (*gen.OpenAPIDoc, string, FetchInfo, []string, error) {
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return f.fetchURL(ctx, input, auth)
	}
	return f.fetchFile(input)
}

func (f *OpenAPIFetcher) fetchFile(path string) (*gen.OpenAPIDoc, string, FetchInfo, []string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", FetchInfo{}, nil, fmt.Errorf("read openapi file: %w", err)
	}
	doc, err := gen.ParseSpec(raw)
	if err != nil {
		return nil, "", FetchInfo{}, nil, err
	}
	sum := sha256.Sum256(raw)
	hexsum := hex.EncodeToString(sum[:])
	info := FetchInfo{Source: "file", File: path, Hash: hexsum}
	return doc, hexsum, info, nil, nil
}

func (f *OpenAPIFetcher) fetchURL(ctx context.Context, rawURL string, auth *AuthConfig) (*gen.OpenAPIDoc, string, FetchInfo, []string, error) {
	warnings := []string{}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", FetchInfo{}, nil, err
	}
	req.Header.Set("Accept-Encoding", "gzip")
	applyAuth(req, auth)
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, "", FetchInfo{}, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, "", FetchInfo{}, nil, fmt.Errorf("openapi fetch status %d", resp.StatusCode)
	}

	var reader io.Reader = io.LimitReader(resp.Body, f.HTTPConfig.MaxBytes+1)
	if strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		gzr, gzErr := gzip.NewReader(reader)
		if gzErr != nil {
			return nil, "", FetchInfo{}, nil, fmt.Errorf("decode gzip openapi: %w", gzErr)
		}
		defer gzr.Close()
		reader = io.LimitReader(gzr, f.HTTPConfig.MaxBytes+1)
	}

	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, "", FetchInfo{}, nil, err
	}
	if int64(len(raw)) > f.HTTPConfig.MaxBytes {
		return nil, "", FetchInfo{}, nil, fmt.Errorf("openapi payload exceeds max bytes")
	}
	doc, err := gen.ParseSpec(raw)
	if err != nil {
		return nil, "", FetchInfo{}, nil, err
	}
	sum := sha256.Sum256(raw)
	hexsum := hex.EncodeToString(sum[:])
	info := FetchInfo{
		Source: "url",
		URL:    canonicalURL(rawURL),
		Hash:   hexsum,
	}
	if resp.Header.Get("Content-Encoding") != "" && !strings.EqualFold(resp.Header.Get("Content-Encoding"), "gzip") {
		warnings = append(warnings, "unexpected content-encoding for openapi response")
	}
	return doc, hexsum, info, warnings, nil
}

func canonicalURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Host = strings.ToLower(u.Host)
	u.Path = strings.TrimSuffix(u.Path, "/")

	keys := make([]string, 0, len(u.Query()))
	for k := range u.Query() {
		keys = append(keys, k)
	}
	sortStrings(keys)
	q := url.Values{}
	for _, k := range keys {
		vals := append([]string(nil), u.Query()[k]...)
		sortStrings(vals)
		for _, v := range vals {
			q.Add(k, v)
		}
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func sortStrings(items []string) {
	if len(items) < 2 {
		return
	}
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j] < items[i] {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

func applyAuth(req *http.Request, auth *AuthConfig) {
	if auth == nil {
		return
	}
	token := auth.EnvValue()
	if token == "" {
		return
	}
	switch auth.Kind {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+token)
	case "api_key":
		if auth.Location == "header" {
			req.Header.Set(auth.Name, token)
		} else if auth.Location == "query" {
			q := req.URL.Query()
			q.Set(auth.Name, token)
			req.URL.RawQuery = q.Encode()
		}
	}
}

func JSONHashCanonical(raw []byte) (string, []byte, error) {
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", nil, err
	}
	canonical, err := utils.CanonicalJSON(decoded)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), canonical, nil
}

func HashBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func bytesTrimSpace(in []byte) []byte {
	return bytes.TrimSpace(in)
}
