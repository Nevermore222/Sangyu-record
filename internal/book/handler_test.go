package book

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestHandlerReturnsLatestArtifactDownload(t *testing.T) {
	projectID := uuid.New()
	repo := &memoryArtifactRepository{saved: Artifact{ID: uuid.New(), ProjectID: projectID, Kind: "pdf", ObjectKey: "memoir.pdf"}}
	catalog := NewCatalog(repo, &memoryArtifactStore{}, "private")
	router := chi.NewRouter()
	NewHandler(catalog).Register(router)
	req := httptest.NewRequest(http.MethodGet, "/projects/"+projectID.String()+"/artifacts/latest", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if got := rec.Body.String(); !strings.Contains(got, "http://localhost:9000/download") {
		t.Fatalf("body = %s", got)
	}
}
