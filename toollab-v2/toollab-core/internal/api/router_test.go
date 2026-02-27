package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"toollab-v2/internal/model"
	"toollab-v2/internal/store"
)

func TestCreateRunPipeline(t *testing.T) {
	tmp := t.TempDir()
	mainGo := `package main
import "net/http"
func main() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {})
}`
	if err := os.WriteFile(filepath.Join(tmp, "main.go"), []byte(mainGo), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := NewServer(store.New())
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body, _ := json.Marshal(map[string]any{
		"source_type": "local_path",
		"local_path":  tmp,
		"llm_enabled": false,
	})
	resp, err := http.Post(ts.URL+"/v1/runs", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status inesperado: %d", resp.StatusCode)
	}
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.RunID == "" {
		t.Fatal("run_id vacío")
	}

	var run model.Run
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r2, err := http.Get(ts.URL + "/v1/runs/" + created.RunID)
		if err != nil {
			t.Fatal(err)
		}
		_ = json.NewDecoder(r2.Body).Decode(&run)
		_ = r2.Body.Close()
		if run.Status == model.RunSucceeded || run.Status == model.RunFailed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if run.Status != model.RunSucceeded {
		t.Fatalf("run no finalizó ok: %s (%s)", run.Status, run.ErrorMessage)
	}

	r3, err := http.Get(ts.URL + "/v1/runs/" + created.RunID + "/model")
	if err != nil {
		t.Fatal(err)
	}
	defer r3.Body.Close()
	if r3.StatusCode != http.StatusOK {
		t.Fatalf("model status inesperado: %d", r3.StatusCode)
	}
}
