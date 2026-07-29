package staff

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/nevermore222/sangyu-record/internal/httpapi"
)

type Application interface {
	LoginDev(context.Context, string) (LoginResult, error)
	LoginWechat(context.Context, string) (LoginResult, error)
	Logout(context.Context, string) error
}

type Handler struct {
	service Application
}

func NewHandler(service Application) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterAuth(router chi.Router) {
	router.Post("/v1/auth/dev", h.loginDev)
	router.Post("/v1/auth/wechat", h.loginWechat)
}

func (h *Handler) RegisterStaff(router chi.Router) {
	router.Get("/me", h.me)
	router.Post("/logout", h.logout)
}

func (h *Handler) loginDev(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DisplayName string `json:"display_name"`
	}
	if !decodeRequest(w, r, &input) {
		return
	}
	result, err := h.service.LoginDev(r.Context(), input.DisplayName)
	h.writeLoginResult(w, result, err)
}

func (h *Handler) loginWechat(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code string `json:"code"`
	}
	if !decodeRequest(w, r, &input) {
		return
	}
	result, err := h.service.LoginWechat(r.Context(), input.Code)
	h.writeLoginResult(w, result, err)
}

func (h *Handler) writeLoginResult(w http.ResponseWriter, result LoginResult, err error) {
	switch {
	case errors.Is(err, ErrValidation):
		writeAuthError(w, http.StatusUnprocessableEntity, "validation_failed", err.Error())
	case errors.Is(err, ErrForbidden):
		writeAuthError(w, http.StatusForbidden, "staff_forbidden", "staff access is not allowed")
	case err != nil:
		writeAuthError(w, http.StatusBadGateway, "authentication_failed", "could not authenticate staff")
	default:
		httpapi.WriteJSON(w, http.StatusCreated, result)
	}
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	value, ok := FromContext(r.Context())
	if !ok {
		writeAuthError(w, http.StatusUnauthorized, "staff_unauthorized", "staff login is required")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, value)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	if err := h.service.Logout(r.Context(), token); err != nil {
		writeAuthError(w, http.StatusInternalServerError, "logout_failed", "could not end staff session")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func decodeRequest(w http.ResponseWriter, r *http.Request, output any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		writeAuthError(w, http.StatusUnprocessableEntity, "invalid_request", err.Error())
		return false
	}
	return true
}
