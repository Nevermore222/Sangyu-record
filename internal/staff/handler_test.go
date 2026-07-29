package staff

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func TestHandlerDevLoginAndCurrentStaff(t *testing.T) {
	service := NewService(newMemoryRepository(), nil, Config{
		Mode: "dev", SessionTTL: time.Hour, SessionSecret: []byte("test-session-secret"),
	}, time.Now)
	handler := NewHandler(service)
	router := chi.NewRouter()
	handler.RegisterAuth(router)
	router.Route("/v1/staff", func(r chi.Router) {
		r.Use(NewMiddleware(service).Handle)
		handler.RegisterStaff(r)
	})

	loginRequest := httptest.NewRequest(http.MethodPost, "/v1/auth/dev", bytes.NewReader([]byte(`{"display_name":"现场采集员"}`)))
	loginResponse := httptest.NewRecorder()
	router.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusCreated {
		t.Fatalf("login status = %d, body = %s", loginResponse.Code, loginResponse.Body.String())
	}
	var login LoginResult
	decodeJSON(t, loginResponse.Body, &login)

	meRequest := httptest.NewRequest(http.MethodGet, "/v1/staff/me", nil)
	meRequest.Header.Set("Authorization", "Bearer "+login.Token)
	meResponse := httptest.NewRecorder()
	router.ServeHTTP(meResponse, meRequest)
	if meResponse.Code != http.StatusOK {
		t.Fatalf("me status = %d, body = %s", meResponse.Code, meResponse.Body.String())
	}
}

func TestHandlerRejectsDevLoginOutsideDevMode(t *testing.T) {
	service := NewService(newMemoryRepository(), fakeExchanger{openID: "allowed"}, Config{
		Mode: "wechat", SessionTTL: time.Hour, SessionSecret: []byte("test-session-secret"),
		AllowedOpenIDs: map[string]struct{}{"allowed": {}},
	}, time.Now)
	router := chi.NewRouter()
	NewHandler(service).RegisterAuth(router)
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/dev", bytes.NewReader([]byte(`{"display_name":"test"}`)))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}
}

func decodeJSON(t *testing.T, reader io.Reader, output any) {
	t.Helper()
	if err := json.NewDecoder(reader).Decode(output); err != nil {
		t.Fatal(err)
	}
}
