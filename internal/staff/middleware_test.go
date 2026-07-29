package staff

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type authenticatorFunc func(context.Context, string) (Staff, error)

func (f authenticatorFunc) Authenticate(ctx context.Context, token string) (Staff, error) {
	return f(ctx, token)
}

func TestMiddlewareRejectsMissingBearerToken(t *testing.T) {
	middleware := NewMiddleware(authenticatorFunc(func(context.Context, string) (Staff, error) {
		t.Fatal("authenticator should not be called")
		return Staff{}, nil
	}))
	handler := middleware.Handle(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("protected handler was called")
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/staff/me", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareAddsAuthenticatedStaffToContext(t *testing.T) {
	want := Staff{DisplayName: "林采集员", State: StateActive}
	middleware := NewMiddleware(authenticatorFunc(func(_ context.Context, token string) (Staff, error) {
		if token != "valid-token" {
			t.Fatalf("token = %q", token)
		}
		return want, nil
	}))
	handler := middleware.Handle(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, ok := FromContext(r.Context())
		if !ok || got.DisplayName != want.DisplayName {
			t.Fatalf("staff = %#v, ok = %v", got, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/v1/staff/me", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d", response.Code)
	}
}
