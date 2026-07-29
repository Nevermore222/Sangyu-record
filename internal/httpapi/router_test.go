package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	NewRouter(Dependencies{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "{\"status\":\"ok\"}\n" {
		t.Fatalf("body = %q", got)
	}
}

func TestProviderRoutesAreRegisteredOutsideStaffPrefix(t *testing.T) {
	deps := Dependencies{RegisterProviderRoutes: func(router chi.Router) {
		router.Post("/v1/provider-callbacks/{kind}/{jobID}", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusAccepted)
		})
	}}
	router := NewRouter(deps)

	req := httptest.NewRequest(http.MethodPost, "/v1/provider-callbacks/media/job-1", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("callback status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/staff/v1/provider-callbacks/media/job-1", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("staff-prefixed callback status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
