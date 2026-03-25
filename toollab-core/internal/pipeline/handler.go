package pipeline

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/devpablocristo/core/http/go/httpjson"
)

type Handler struct {
	orchestrator *Orchestrator
}

func NewHandler(o *Orchestrator) *Handler {
	return &Handler{orchestrator: o}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.analyzeSSE)
	return r
}

func (h *Handler) analyzeSSE(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "target_id")
	if targetID == "" {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "target_id is required")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL", "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	emit := func(ev ProgressEvent) {
		data, _ := json.Marshal(ev)
		fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
		flusher.Flush()
	}

	lang := r.URL.Query().Get("lang")
	if lang != "es" {
		lang = "en"
	}

	result, err := h.orchestrator.Analyze(r.Context(), targetID, lang, emit)
	if err != nil {
		errData, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", errData)
		flusher.Flush()
		return
	}

	resultData, _ := json.Marshal(result)
	fmt.Fprintf(w, "event: result\ndata: %s\n\n", resultData)
	flusher.Flush()
}
