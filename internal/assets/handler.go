package assets

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
	Initiate(context.Context, InitiateInput) (UploadTicket, error)
	Get(context.Context, uuid.UUID) (Asset, error)
	Complete(context.Context, uuid.UUID, string) (Asset, error)
	RenewUpload(context.Context, uuid.UUID) (UploadTicket, error)
	DeletePending(context.Context, uuid.UUID) error
	ListByVisit(context.Context, uuid.UUID) ([]Asset, error)
}

type VisitAuthorizer interface {
	Authorize(context.Context, uuid.UUID, uuid.UUID) error
}

type Handler struct {
	service Application
	visits  VisitAuthorizer
}

func NewHandler(service Application, authorizers ...VisitAuthorizer) *Handler {
	handler := &Handler{service: service}
	if len(authorizers) > 0 {
		handler.visits = authorizers[0]
	}
	return handler
}

func (h *Handler) Routes() http.Handler {
	router := chi.NewRouter()
	h.Register(router)
	return router
}

func (h *Handler) Register(router chi.Router) {
	router.Post("/projects/{projectID}/assets:initiate", h.initiate)
	router.Post("/assets/{assetID}:complete", h.complete)
	router.Get("/visits/{visitID}/assets", h.listByVisit)
	router.Post("/assets/{assetID}:renew-upload", h.renewUpload)
	router.Delete("/assets/{assetID}", h.deletePending)
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
	if input.VisitID != nil && h.visits != nil {
		current, ok := staff.FromContext(r.Context())
		if !ok {
			writeAssetError(w, http.StatusUnauthorized, "staff_unauthorized", "staff login is required")
			return
		}
		if err := h.visits.Authorize(r.Context(), *input.VisitID, current.ID); err != nil {
			writeAssetError(w, http.StatusNotFound, "visit_not_found", "visit was not found")
			return
		}
	}

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

func (h *Handler) listByVisit(w http.ResponseWriter, r *http.Request) {
	visitID, err := uuid.Parse(chi.URLParam(r, "visitID"))
	if err != nil {
		writeAssetError(w, http.StatusUnprocessableEntity, "invalid_visit_id", "visit ID must be a UUID")
		return
	}
	if h.visits != nil {
		current, ok := staff.FromContext(r.Context())
		if !ok {
			writeAssetError(w, http.StatusUnauthorized, "staff_unauthorized", "staff login is required")
			return
		}
		if err := h.visits.Authorize(r.Context(), visitID, current.ID); err != nil {
			writeAssetError(w, http.StatusNotFound, "visit_not_found", "visit was not found")
			return
		}
	}
	items, err := h.service.ListByVisit(r.Context(), visitID)
	if err != nil {
		writeAssetError(w, http.StatusInternalServerError, "internal_error", "could not list assets")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) renewUpload(w http.ResponseWriter, r *http.Request) {
	assetID, err := uuid.Parse(chi.URLParam(r, "assetID"))
	if err != nil {
		writeAssetError(w, http.StatusUnprocessableEntity, "invalid_asset_id", "asset ID must be a UUID")
		return
	}
	if !h.authorizeAsset(w, r, assetID) {
		return
	}
	ticket, err := h.service.RenewUpload(r.Context(), assetID)
	switch {
	case errors.Is(err, ErrNotFound):
		writeAssetError(w, http.StatusNotFound, "asset_not_found", "asset was not found")
	case errors.Is(err, ErrInvalidState):
		writeAssetError(w, http.StatusConflict, "asset_state_conflict", err.Error())
	case err != nil:
		writeAssetError(w, http.StatusInternalServerError, "internal_error", "could not renew upload")
	default:
		httpapi.WriteJSON(w, http.StatusOK, ticket)
	}
}

func (h *Handler) deletePending(w http.ResponseWriter, r *http.Request) {
	assetID, err := uuid.Parse(chi.URLParam(r, "assetID"))
	if err != nil {
		writeAssetError(w, http.StatusUnprocessableEntity, "invalid_asset_id", "asset ID must be a UUID")
		return
	}
	if !h.authorizeAsset(w, r, assetID) {
		return
	}
	err = h.service.DeletePending(r.Context(), assetID)
	switch {
	case errors.Is(err, ErrNotFound):
		writeAssetError(w, http.StatusNotFound, "asset_not_found", "asset was not found")
	case errors.Is(err, ErrInvalidState):
		writeAssetError(w, http.StatusConflict, "asset_state_conflict", err.Error())
	case err != nil:
		writeAssetError(w, http.StatusInternalServerError, "internal_error", "could not delete asset")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) complete(w http.ResponseWriter, r *http.Request) {
	assetID, err := uuid.Parse(chi.URLParam(r, "assetID"))
	if err != nil {
		writeAssetError(w, http.StatusUnprocessableEntity, "invalid_asset_id", "asset ID must be a UUID")
		return
	}
	if !h.authorizeAsset(w, r, assetID) {
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

func (h *Handler) authorizeAsset(w http.ResponseWriter, r *http.Request, assetID uuid.UUID) bool {
	if h.visits == nil {
		return true
	}
	current, ok := staff.FromContext(r.Context())
	if !ok {
		writeAssetError(w, http.StatusUnauthorized, "staff_unauthorized", "staff login is required")
		return false
	}
	asset, err := h.service.Get(r.Context(), assetID)
	if errors.Is(err, ErrNotFound) {
		writeAssetError(w, http.StatusNotFound, "asset_not_found", "asset was not found")
		return false
	}
	if err != nil {
		writeAssetError(w, http.StatusInternalServerError, "internal_error", "could not authorize asset")
		return false
	}
	if asset.VisitID == nil {
		return true
	}
	if err := h.visits.Authorize(r.Context(), *asset.VisitID, current.ID); err != nil {
		writeAssetError(w, http.StatusNotFound, "asset_not_found", "asset was not found")
		return false
	}
	return true
}

func writeAssetError(w http.ResponseWriter, status int, code, message string) {
	httpapi.WriteJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
