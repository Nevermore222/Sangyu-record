package projects

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/staff"
)

func TestHandlerCreatesProject(t *testing.T) {
	service := NewService(newMemoryRepository(), DeterministicPlanner{})
	handler := NewHandler(service).Routes()
	body := []byte(`{
		"display_name":"林奶奶",
		"birth_year":1948,
		"birth_place":"江苏苏州",
		"long_term_residence":"江苏苏州",
		"primary_occupation":"纺织工人",
		"target_edition":"standard"
	}`)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var detail ProjectDetail
	if err := json.NewDecoder(rec.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	if detail.DisplayName != "林奶奶" || len(detail.CollectionPlan) != 7 {
		t.Fatalf("detail = %#v", detail)
	}
}

func TestHandlerRegistersStaffProjectPath(t *testing.T) {
	service := NewService(newMemoryRepository(), DeterministicPlanner{})
	router := chi.NewRouter()
	NewHandler(service).Register(router)
	req := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewReader([]byte(`{
		"display_name":"林奶奶",
		"birth_year":1948,
		"birth_place":"苏州",
		"long_term_residence":"苏州",
		"target_edition":"brief"
	}`)))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestHandlerRejectsUnknownCreateField(t *testing.T) {
	service := NewService(newMemoryRepository(), DeterministicPlanner{})
	handler := NewHandler(service).Routes()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{
		"display_name":"林奶奶",
		"birth_year":1948,
		"birth_place":"苏州",
		"long_term_residence":"苏州",
		"target_edition":"brief",
		"unexpected":true
	}`)))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
}

func TestHandlerReturnsNotFound(t *testing.T) {
	service := NewService(newMemoryRepository(), DeterministicPlanner{})
	handler := NewHandler(service).Routes()
	req := httptest.NewRequest(http.MethodGet, "/00000000-0000-0000-0000-000000000001", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

type authenticatedStaff struct {
	value staff.Staff
}

func (a authenticatedStaff) Authenticate(_ context.Context, _ string) (staff.Staff, error) {
	return a.value, nil
}

func TestHandlerCreatesOwnedProjectListsItAndConfirmsConsent(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(repo, DeterministicPlanner{})
	owner := staff.Staff{ID: uuid.New(), DisplayName: "Authenticated Collector", State: staff.StateActive}
	router := chi.NewRouter()
	router.Use(staff.NewMiddleware(authenticatedStaff{value: owner}).Handle)
	NewHandler(service).Register(router)

	create := httptest.NewRequest(http.MethodPost, "/projects", bytes.NewReader([]byte(`{
		"display_name":"Owner Test",
		"birth_year":1948,
		"birth_place":"Suzhou",
		"long_term_residence":"Suzhou",
		"target_edition":"brief"
	}`)))
	create.Header.Set("Authorization", "Bearer test-token")
	createdResponse := httptest.NewRecorder()
	router.ServeHTTP(createdResponse, create)
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", createdResponse.Code, createdResponse.Body.String())
	}
	var created ProjectDetail
	if err := json.NewDecoder(createdResponse.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.OwnerStaffID != owner.ID {
		t.Fatalf("owner = %s, want %s", created.OwnerStaffID, owner.ID)
	}

	list := httptest.NewRequest(http.MethodGet, "/projects?limit=20", nil)
	list.Header.Set("Authorization", "Bearer test-token")
	listResponse := httptest.NewRecorder()
	router.ServeHTTP(listResponse, list)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResponse.Code, listResponse.Body.String())
	}
	var page Page
	if err := json.NewDecoder(listResponse.Body).Decode(&page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != created.ID {
		t.Fatalf("page = %#v", page)
	}

	consent := httptest.NewRequest(
		http.MethodPost,
		"/projects/"+created.ID.String()+"/consents",
		bytes.NewReader([]byte(`{"confirmed_by":"elder"}`)),
	)
	consent.Header.Set("Authorization", "Bearer test-token")
	consentResponse := httptest.NewRecorder()
	router.ServeHTTP(consentResponse, consent)
	if consentResponse.Code != http.StatusCreated {
		t.Fatalf("consent status = %d, body = %s", consentResponse.Code, consentResponse.Body.String())
	}
}
