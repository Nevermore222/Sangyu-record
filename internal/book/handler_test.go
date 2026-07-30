package book

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/staff"
)

func TestHandlerReturnsLatestArtifactDownload(t *testing.T) {
	projectID := uuid.New()
	repo := &memoryArtifactRepository{saved: Artifact{ID: uuid.New(), ProjectID: projectID, Kind: "pdf", ObjectKey: "memoir.pdf"}}
	catalog := NewCatalog(repo, &memoryArtifactStore{}, "private")
	router := chi.NewRouter()
	owner := staff.Staff{ID: uuid.New(), State: staff.StateActive}
	router.Use(staff.NewMiddleware(bookTestAuthenticator{value: owner}).Handle)
	NewHandler(catalog).Register(router)
	req := httptest.NewRequest(http.MethodGet, "/projects/"+projectID.String()+"/artifacts/latest", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); !strings.Contains(got, "http://localhost:9000/download") {
		t.Fatalf("body = %s", got)
	}
}

type bookTestAuthenticator struct{ value staff.Staff }

func (a bookTestAuthenticator) Authenticate(_ context.Context, _ string) (staff.Staff, error) {
	return a.value, nil
}
