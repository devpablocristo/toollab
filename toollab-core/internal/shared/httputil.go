package shared

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
)

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, map[string]string{"error": msg})
}

func ErrorStatus(err error) int {
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrInvalidInput):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// RewriteHost applies TOOLLAB_HOST_REWRITE env var to URLs.
// Format: "from=to" e.g. "localhost=host.docker.internal"
func RewriteHost(u string) string {
	rewrite := os.Getenv("TOOLLAB_HOST_REWRITE")
	if rewrite == "" {
		return u
	}
	parts := strings.SplitN(rewrite, "=", 2)
	if len(parts) != 2 {
		return u
	}
	return strings.Replace(u, parts[0], parts[1], 1)
}
