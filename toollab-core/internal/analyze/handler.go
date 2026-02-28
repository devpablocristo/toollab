package analyze

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"toollab-core/internal/shared"
)

type Handler struct {
	orchestrator *Orchestrator
}

func NewHandler(orchestrator *Orchestrator) *Handler {
	return &Handler{orchestrator: orchestrator}
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Post("/", h.analyzeSSE)
	return r
}

func (h *Handler) analyzeSSE(w http.ResponseWriter, r *http.Request) {
	targetID := chi.URLParam(r, "target_id")
	if targetID == "" {
		shared.WriteError(w, http.StatusBadRequest, "target_id is required")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		shared.WriteError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	emit := func(event ProgressEvent) {
		data, _ := json.Marshal(event)
		fmt.Fprintf(w, "event: progress\ndata: %s\n\n", data)
		flusher.Flush()
	}

	result, err := h.orchestrator.Analyze(r.Context(), targetID, ProgressEmitter(emit))

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
