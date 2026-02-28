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

func HostRewrite() func(string) string {
	rw := os.Getenv("TOOLLAB_HOST_REWRITE")
	if rw == "" {
		return nil
	}
	parts := strings.SplitN(rw, "=", 2)
	if len(parts) != 2 {
		return nil
	}
	return func(url string) string {
		return strings.Replace(url, parts[0], parts[1], 1)
	}
}

func ErrorStatus(err error) int {
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrConflict):
		return http.StatusConflict
	case errors.Is(err, ErrInvalidInput):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}
