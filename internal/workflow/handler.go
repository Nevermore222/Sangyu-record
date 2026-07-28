package workflow

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/httpapi"
)

type Application interface {
	Start(context.Context, uuid.UUID) (Run, error)
	Latest(context.Context, uuid.UUID) (Run, error)
}

type Handler struct {
	service Application
}

func NewHandler(service Application) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(router chi.Router) {
	router.Post("/projects/{projectID}/workflow:start", h.start)
	router.Get("/projects/{projectID}/workflow", h.latest)
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	projectID, ok := workflowProjectID(w, r)
	if !ok {
		return
	}
	run, err := h.service.Start(r.Context(), projectID)
	if errors.Is(err, ErrInsufficientAssets) {
		writeWorkflowError(w, http.StatusConflict, "insufficient_assets", err.Error())
		return
	}
	if err != nil {
		writeWorkflowError(w, http.StatusInternalServerError, "internal_error", "could not start workflow")
		return
	}
	httpapi.WriteJSON(w, http.StatusAccepted, run)
}

func (h *Handler) latest(w http.ResponseWriter, r *http.Request) {
	projectID, ok := workflowProjectID(w, r)
	if !ok {
		return
	}
	run, err := h.service.Latest(r.Context(), projectID)
	if errors.Is(err, ErrRunNotFound) {
		writeWorkflowError(w, http.StatusNotFound, "workflow_not_found", err.Error())
		return
	}
	if err != nil {
		writeWorkflowError(w, http.StatusInternalServerError, "internal_error", "could not load workflow")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, run)
}

func workflowProjectID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeWorkflowError(w, http.StatusUnprocessableEntity, "invalid_project_id", "project ID must be a UUID")
		return uuid.Nil, false
	}
	return projectID, true
}

func writeWorkflowError(w http.ResponseWriter, status int, code, message string) {
	httpapi.WriteJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
