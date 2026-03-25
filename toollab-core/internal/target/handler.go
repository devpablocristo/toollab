package target

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/devpablocristo/core/errors/go/domainerr"
	"github.com/devpablocristo/core/http/go/httpjson"

	"toollab-core/internal/target/handler/dto"
	"toollab-core/internal/target/usecases/domain"
)

type Handler struct{ svc *Service }

func New(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.create)
	r.Get("/", h.list)
	r.Get("/{target_id}", h.get)
	r.Delete("/{target_id}", h.delete)
	return r
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	t, err := h.svc.Create(req.Name, req.Description, req.Source, req.RuntimeHint)
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusCreated, t)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.List()
	if err != nil {
		httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
		return
	}
	if items == nil {
		items = []domain.Target{}
	}
	httpjson.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	t, err := h.svc.Get(chi.URLParam(r, "target_id"))
	if err != nil {
		writeError(w, err)
		return
	}
	httpjson.WriteJSON(w, http.StatusOK, t)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.Delete(chi.URLParam(r, "target_id")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeError(w http.ResponseWriter, err error) {
	if domainerr.IsNotFound(err) {
		httpjson.WriteError(w, http.StatusNotFound, "NOT_FOUND", "not found")
		return
	}
	if domainerr.IsValidation(err) {
		httpjson.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}
	httpjson.WriteError(w, http.StatusInternalServerError, "INTERNAL", "internal error")
}
