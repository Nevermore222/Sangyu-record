package book

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/httpapi"
	"github.com/nevermore222/sangyu-record/internal/staff"
)

type ArtifactApplication interface {
	Latest(context.Context, uuid.UUID, uuid.UUID) (Artifact, error)
}

type Handler struct {
	catalog ArtifactApplication
}

func NewHandler(catalog ArtifactApplication) *Handler {
	return &Handler{catalog: catalog}
}

func (h *Handler) Register(router chi.Router) {
	router.Get("/projects/{projectID}/artifacts/latest", h.latest)
}

func (h *Handler) latest(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeBookError(w, http.StatusUnprocessableEntity, "invalid_project_id", "project ID must be a UUID")
		return
	}
	current, ok := staff.FromContext(r.Context())
	if !ok {
		writeBookError(w, http.StatusUnauthorized, "staff_unauthorized", "staff login is required")
		return
	}
	artifact, err := h.catalog.Latest(r.Context(), projectID, current.ID)
	if errors.Is(err, ErrArtifactNotFound) {
		writeBookError(w, http.StatusNotFound, "artifact_not_found", "PDF artifact was not found")
		return
	}
	if err != nil {
		writeBookError(w, http.StatusInternalServerError, "internal_error", "could not create download link")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, artifact)
}

func writeBookError(w http.ResponseWriter, status int, code, message string) {
	httpapi.WriteJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
