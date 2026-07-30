package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/httpapi"
	"github.com/nevermore222/sangyu-record/internal/staff"
)

type Application interface {
	Start(context.Context, uuid.UUID) (Run, error)
	Latest(context.Context, uuid.UUID) (Run, error)
	Finalize(context.Context, uuid.UUID, uuid.UUID, FinalizeInput) (Run, error)
}

type Handler struct {
	service Application
}

func NewHandler(service Application) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(router chi.Router) {
	router.Post("/projects/{projectID}/workflow:start", h.start)
	router.Post("/projects/{projectID}:finalize", h.finalize)
	router.Get("/projects/{projectID}/workflow", h.latest)
}

func (h *Handler) finalize(w http.ResponseWriter, r *http.Request) {
	projectID, ok := workflowProjectID(w, r)
	if !ok {
		return
	}
	current, ok := staff.FromContext(r.Context())
	if !ok {
		writeWorkflowError(w, http.StatusUnauthorized, "staff_unauthorized", "staff login is required")
		return
	}
	var input FinalizeInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeWorkflowError(w, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	run, err := h.service.Finalize(r.Context(), projectID, current.ID, input)
	switch {
	case errors.Is(err, ErrConfirmationRequired):
		writeWorkflowError(w, http.StatusUnprocessableEntity, "confirmation_required", err.Error())
	case errors.Is(err, ErrProjectNotFound):
		writeWorkflowError(w, http.StatusNotFound, "project_not_found", err.Error())
	case errors.Is(err, ErrConsentRequired):
		writeWorkflowError(w, http.StatusConflict, "consent_required", err.Error())
	case errors.Is(err, ErrDraftVisitExists):
		writeWorkflowError(w, http.StatusConflict, "draft_visit_exists", err.Error())
	case errors.Is(err, ErrInsufficientAssets):
		writeWorkflowError(w, http.StatusConflict, "insufficient_assets", err.Error())
	case err != nil:
		writeWorkflowError(w, http.StatusInternalServerError, "internal_error", "could not finalize project")
	default:
		httpapi.WriteJSON(w, http.StatusAccepted, run)
	}
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
