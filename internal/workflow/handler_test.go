package workflow

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
