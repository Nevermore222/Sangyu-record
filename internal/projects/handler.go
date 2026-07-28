package projects

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/httpapi"
)

type Application interface {
	Create(context.Context, CreateInput) (ProjectDetail, error)
	Get(context.Context, uuid.UUID) (ProjectDetail, error)
}

type Handler struct {
	service Application
}

func NewHandler(service Application) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Routes() http.Handler {
	router := chi.NewRouter()
	router.Post("/", h.create)
	router.Get("/{projectID}", h.get)
	return router
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var input CreateInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}

	detail, err := h.service.Create(r.Context(), input)
	if errors.Is(err, ErrValidation) {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not create project")
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, detail)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_project_id", "project ID must be a UUID")
		return
	}

	detail, err := h.service.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "project_not_found", "project was not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load project")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, detail)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	httpapi.WriteJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
