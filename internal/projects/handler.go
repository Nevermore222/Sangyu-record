package projects

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/httpapi"
	"github.com/nevermore222/sangyu-record/internal/staff"
)

type Application interface {
	Create(context.Context, CreateInput) (ProjectDetail, error)
	Get(context.Context, uuid.UUID) (ProjectDetail, error)
	GetOwned(context.Context, uuid.UUID, uuid.UUID) (ProjectDetail, error)
	List(context.Context, ListInput) (Page, error)
	Dashboard(context.Context, uuid.UUID) (Dashboard, error)
	ConfirmConsent(context.Context, uuid.UUID, uuid.UUID, ConfirmConsentInput) (Consent, error)
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

func (h *Handler) Register(router chi.Router) {
	router.Get("/dashboard", h.dashboard)
	router.Get("/projects", h.list)
	router.Post("/projects", h.create)
	router.Get("/projects/{projectID}", h.get)
	router.Post("/projects/{projectID}/consents", h.confirmConsent)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var input CreateInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	if current, ok := staff.FromContext(r.Context()); ok {
		input.OwnerStaffID = current.ID
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

	var detail ProjectDetail
	if current, ok := staff.FromContext(r.Context()); ok {
		detail, err = h.service.GetOwned(r.Context(), id, current.ID)
	} else {
		detail, err = h.service.Get(r.Context(), id)
	}
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

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	current, ok := staff.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "staff_unauthorized", "staff login is required")
		return
	}
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 {
			writeError(w, http.StatusUnprocessableEntity, "invalid_limit", "limit must be a positive integer")
			return
		}
		limit = value
	}
	page, err := h.service.List(r.Context(), ListInput{
		OwnerStaffID: current.ID,
		Query:        r.URL.Query().Get("query"),
		State:        State(r.URL.Query().Get("state")),
		Cursor:       r.URL.Query().Get("cursor"),
		Limit:        limit,
	})
	if errors.Is(err, ErrValidation) {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not list projects")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, page)
}

func (h *Handler) dashboard(w http.ResponseWriter, r *http.Request) {
	current, ok := staff.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "staff_unauthorized", "staff login is required")
		return
	}
	dashboard, err := h.service.Dashboard(r.Context(), current.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load dashboard")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, dashboard)
}

func (h *Handler) confirmConsent(w http.ResponseWriter, r *http.Request) {
	current, ok := staff.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "staff_unauthorized", "staff login is required")
		return
	}
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_project_id", "project ID must be a UUID")
		return
	}
	var input ConfirmConsentInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	consent, err := h.service.ConfirmConsent(r.Context(), projectID, current.ID, input)
	if errors.Is(err, ErrValidation) {
		writeError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		return
	}
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "project_not_found", "project was not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "could not confirm consent")
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, consent)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	httpapi.WriteJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
