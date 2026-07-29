package visits

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
	Create(context.Context, CreateInput) (Visit, error)
	Get(context.Context, uuid.UUID, uuid.UUID) (Visit, error)
	List(context.Context, uuid.UUID, uuid.UUID) ([]Visit, error)
	Update(context.Context, uuid.UUID, uuid.UUID, UpdateInput) (Visit, error)
}

type Handler struct {
	service Application
}

func NewHandler(service Application) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Register(router chi.Router) {
	router.Post("/projects/{projectID}/visits", h.create)
	router.Get("/projects/{projectID}/visits", h.list)
	router.Get("/visits/{visitID}", h.get)
	router.Patch("/visits/{visitID}", h.update)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	current, ok := staff.FromContext(r.Context())
	if !ok {
		writeVisitError(w, http.StatusUnauthorized, "staff_unauthorized", "staff login is required")
		return
	}
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeVisitError(w, http.StatusUnprocessableEntity, "invalid_project_id", "project ID must be a UUID")
		return
	}
	var input CreateInput
	if !decodeVisitRequest(w, r, &input) {
		return
	}
	input.ProjectID = projectID
	input.StaffID = current.ID
	value, err := h.service.Create(r.Context(), input)
	h.writeResult(w, http.StatusCreated, value, err)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	current, ok := staff.FromContext(r.Context())
	if !ok {
		writeVisitError(w, http.StatusUnauthorized, "staff_unauthorized", "staff login is required")
		return
	}
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeVisitError(w, http.StatusUnprocessableEntity, "invalid_project_id", "project ID must be a UUID")
		return
	}
	items, err := h.service.List(r.Context(), projectID, current.ID)
	if errors.Is(err, ErrNotFound) {
		writeVisitError(w, http.StatusNotFound, "project_not_found", "project was not found")
		return
	}
	if err != nil {
		writeVisitError(w, http.StatusInternalServerError, "internal_error", "could not list visits")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	current, ok := staff.FromContext(r.Context())
	if !ok {
		writeVisitError(w, http.StatusUnauthorized, "staff_unauthorized", "staff login is required")
		return
	}
	visitID, err := uuid.Parse(chi.URLParam(r, "visitID"))
	if err != nil {
		writeVisitError(w, http.StatusUnprocessableEntity, "invalid_visit_id", "visit ID must be a UUID")
		return
	}
	value, err := h.service.Get(r.Context(), visitID, current.ID)
	h.writeResult(w, http.StatusOK, value, err)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	current, ok := staff.FromContext(r.Context())
	if !ok {
		writeVisitError(w, http.StatusUnauthorized, "staff_unauthorized", "staff login is required")
		return
	}
	visitID, err := uuid.Parse(chi.URLParam(r, "visitID"))
	if err != nil {
		writeVisitError(w, http.StatusUnprocessableEntity, "invalid_visit_id", "visit ID must be a UUID")
		return
	}
	var input UpdateInput
	if !decodeVisitRequest(w, r, &input) {
		return
	}
	value, err := h.service.Update(r.Context(), visitID, current.ID, input)
	h.writeResult(w, http.StatusOK, value, err)
}

func (h *Handler) writeResult(w http.ResponseWriter, status int, value Visit, err error) {
	switch {
	case errors.Is(err, ErrValidation):
		writeVisitError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
	case errors.Is(err, ErrConsentRequired):
		writeVisitError(w, http.StatusConflict, "consent_required", err.Error())
	case errors.Is(err, ErrInvalidState):
		writeVisitError(w, http.StatusConflict, "visit_state_conflict", err.Error())
	case errors.Is(err, ErrNotFound):
		writeVisitError(w, http.StatusNotFound, "visit_not_found", "visit was not found")
	case err != nil:
		writeVisitError(w, http.StatusInternalServerError, "internal_error", "could not save visit")
	default:
		httpapi.WriteJSON(w, status, value)
	}
}

func decodeVisitRequest(w http.ResponseWriter, r *http.Request, output any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		writeVisitError(w, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return false
	}
	return true
}

func writeVisitError(w http.ResponseWriter, status int, code, message string) {
	httpapi.WriteJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
