package workflow

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

func TestHandlerStartsWorkflow(t *testing.T) {
	repo := &memoryRunRepository{}
	service := NewService(repo, &memoryQueue{})
	router := chi.NewRouter()
	NewHandler(service).Register(router)
	projectID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/projects/"+projectID.String()+"/workflow:start", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

type workflowTestAuthenticator struct {
	value staff.Staff
}

func (a workflowTestAuthenticator) Authenticate(_ context.Context, _ string) (staff.Staff, error) {
	return a.value, nil
}

func TestHandlerFinalizesConfirmedProject(t *testing.T) {
	owner := staff.Staff{ID: uuid.New(), State: staff.StateActive}
	repo := &memoryRunRepository{finalizeCreated: true}
	router := chi.NewRouter()
	router.Use(staff.NewMiddleware(workflowTestAuthenticator{value: owner}).Handle)
	NewHandler(NewService(repo, &memoryQueue{})).Register(router)
	projectID := uuid.New()
	request := httptest.NewRequest(
		http.MethodPost,
		"/projects/"+projectID.String()+":finalize",
		bytes.NewReader([]byte(`{"confirm_materials_ready":true}`)),
	)
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if repo.input.ProjectID != projectID {
		t.Fatalf("project ID = %s, want %s", repo.input.ProjectID, projectID)
	}
}
