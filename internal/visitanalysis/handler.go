package visitanalysis

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/httpapi"
	"github.com/nevermore222/sangyu-record/internal/staff"
	"github.com/nevermore222/sangyu-record/internal/visits"
	"github.com/nevermore222/sangyu-record/internal/workflow"
)

type Application interface {
	Submit(context.Context, uuid.UUID, uuid.UUID) (workflow.Run, error)
	Retry(context.Context, uuid.UUID, uuid.UUID) (workflow.NodePayload, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (Analysis, error)
}

type Handler struct {
	service Application
}

func NewHandler(service Application) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(router chi.Router) {
	router.Post("/visits/{visitID}:submit", h.submit)
	router.Post("/visits/{visitID}:retry", h.retry)
	router.Get("/visits/{visitID}/analysis", h.get)
}

func (h *Handler) submit(w http.ResponseWriter, r *http.Request) {
	visitID, current, ok := analysisRequestContext(w, r)
	if !ok {
		return
	}
	run, err := h.service.Submit(r.Context(), visitID, current.ID)
	if h.writeError(w, err) {
		return
	}
	httpapi.WriteJSON(w, http.StatusAccepted, run)
}

func (h *Handler) retry(w http.ResponseWriter, r *http.Request) {
	visitID, current, ok := analysisRequestContext(w, r)
	if !ok {
		return
	}
	payload, err := h.service.Retry(r.Context(), visitID, current.ID)
	if h.writeError(w, err) {
		return
	}
	httpapi.WriteJSON(w, http.StatusAccepted, payload)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	visitID, current, ok := analysisRequestContext(w, r)
	if !ok {
		return
	}
	analysis, err := h.service.Get(r.Context(), visitID, current.ID)
	if h.writeError(w, err) {
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, analysis)
}

func (h *Handler) writeError(w http.ResponseWriter, err error) bool {
	switch {
	case err == nil:
		return false
	case IsNotFound(err):
		writeAnalysisError(w, http.StatusNotFound, "analysis_not_found", "visit or analysis was not found")
	case errors.Is(err, ErrInsufficientAssets):
		writeAnalysisError(w, http.StatusConflict, "insufficient_assets", err.Error())
	case errors.Is(err, ErrInvalidState), errors.Is(err, visits.ErrInvalidState):
		writeAnalysisError(w, http.StatusConflict, "visit_state_conflict", err.Error())
	case errors.Is(err, ErrValidation):
		writeAnalysisError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
	default:
		writeAnalysisError(w, http.StatusInternalServerError, "internal_error", "could not process visit analysis")
	}
	return true
}

func analysisRequestContext(w http.ResponseWriter, r *http.Request) (uuid.UUID, staff.Staff, bool) {
	current, ok := staff.FromContext(r.Context())
	if !ok {
		writeAnalysisError(w, http.StatusUnauthorized, "staff_unauthorized", "staff login is required")
		return uuid.Nil, staff.Staff{}, false
	}
	visitID, err := uuid.Parse(chi.URLParam(r, "visitID"))
	if err != nil {
		writeAnalysisError(w, http.StatusUnprocessableEntity, "invalid_visit_id", "visit ID must be a UUID")
		return uuid.Nil, staff.Staff{}, false
	}
	return visitID, current, true
}

func writeAnalysisError(w http.ResponseWriter, status int, code, message string) {
	httpapi.WriteJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
