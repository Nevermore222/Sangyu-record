package assets

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
	Initiate(context.Context, InitiateInput) (UploadTicket, error)
	Complete(context.Context, uuid.UUID, string) (Asset, error)
}

type Handler struct {
	service Application
}

func NewHandler(service Application) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Routes() http.Handler {
	router := chi.NewRouter()
	h.Register(router)
	return router
}

func (h *Handler) Register(router chi.Router) {
	router.Post("/projects/{projectID}/assets:initiate", h.initiate)
	router.Post("/assets/{assetID}:complete", h.complete)
}

func (h *Handler) initiate(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
	if err != nil {
		writeAssetError(w, http.StatusUnprocessableEntity, "invalid_project_id", "project ID must be a UUID")
		return
	}
	var input InitiateInput
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeAssetError(w, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}
	input.ProjectID = projectID

	ticket, err := h.service.Initiate(r.Context(), input)
	if errors.Is(err, ErrValidation) || errors.Is(err, ErrUnsupportedContentType) {
		writeAssetError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
		return
	}
	if err != nil {
		writeAssetError(w, http.StatusInternalServerError, "internal_error", "could not initiate upload")
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, ticket)
}

func (h *Handler) complete(w http.ResponseWriter, r *http.Request) {
	assetID, err := uuid.Parse(chi.URLParam(r, "assetID"))
	if err != nil {
		writeAssetError(w, http.StatusUnprocessableEntity, "invalid_asset_id", "asset ID must be a UUID")
		return
	}
	var input struct {
		SHA256 string `json:"sha256"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeAssetError(w, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return
	}

	asset, err := h.service.Complete(r.Context(), assetID, input.SHA256)
	switch {
	case errors.Is(err, ErrValidation), errors.Is(err, ErrUploadMismatch):
		writeAssetError(w, http.StatusUnprocessableEntity, "upload_invalid", err.Error())
	case errors.Is(err, ErrNotFound):
		writeAssetError(w, http.StatusNotFound, "asset_not_found", "asset was not found")
	case errors.Is(err, ErrHashConflict):
		writeAssetError(w, http.StatusConflict, "asset_hash_conflict", err.Error())
	case err != nil:
		writeAssetError(w, http.StatusInternalServerError, "internal_error", "could not complete upload")
	default:
		httpapi.WriteJSON(w, http.StatusOK, asset)
	}
}

func writeAssetError(w http.ResponseWriter, status int, code, message string) {
	httpapi.WriteJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
