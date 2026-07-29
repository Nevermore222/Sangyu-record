package visits

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/staff"
)

type testAuthenticator struct {
	value staff.Staff
}

func (a testAuthenticator) Authenticate(_ context.Context, _ string) (staff.Staff, error) {
	return a.value, nil
}

func TestHandlerRequiresConsentBeforeCreatingVisit(t *testing.T) {
	owner := staff.Staff{ID: uuid.New(), DisplayName: "Visit Collector", State: staff.StateActive}
	router := chi.NewRouter()
	router.Use(staff.NewMiddleware(testAuthenticator{value: owner}).Handle)
	NewHandler(NewService(newMemoryRepository(), consentChecker{}, false)).Register(router)

	request := httptest.NewRequest(
		http.MethodPost,
		"/projects/"+uuid.NewString()+"/visits",
		bytes.NewReader([]byte(`{"location":"Care Home"}`)),
	)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestHandlerCreatesAndListsVisit(t *testing.T) {
	owner := staff.Staff{ID: uuid.New(), DisplayName: "Visit Collector", State: staff.StateActive}
	projectID := uuid.New()
	service := NewService(newMemoryRepository(), consentChecker{allowed: true}, false)
	router := chi.NewRouter()
	router.Use(staff.NewMiddleware(testAuthenticator{value: owner}).Handle)
	NewHandler(service).Register(router)

	create := httptest.NewRequest(
		http.MethodPost,
		"/projects/"+projectID.String()+"/visits",
		bytes.NewReader([]byte(`{"location":"City Park"}`)),
	)
	create.Header.Set("Authorization", "Bearer test-token")
	created := httptest.NewRecorder()
	router.ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", created.Code, created.Body.String())
	}

	list := httptest.NewRequest(http.MethodGet, "/projects/"+projectID.String()+"/visits", nil)
	list.Header.Set("Authorization", "Bearer test-token")
	listed := httptest.NewRecorder()
	router.ServeHTTP(listed, list)
	if listed.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listed.Code, listed.Body.String())
	}
}
