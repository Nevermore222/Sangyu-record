package assets

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestHandlerInitiatesUpload(t *testing.T) {
	service := NewService(newMemoryRepository(), &fakeObjectStore{}, "private")
	handler := NewHandler(service).Routes()
	projectID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/projects/"+projectID.String()+"/assets:initiate", bytes.NewReader([]byte(`{
		"kind":"photo",
		"filename":"family.jpg",
		"content_type":"image/jpeg",
		"size_bytes":100
	}`)))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var ticket UploadTicket
	if err := json.NewDecoder(rec.Body).Decode(&ticket); err != nil {
		t.Fatal(err)
	}
	if ticket.AssetID == uuid.Nil || ticket.UploadURL == "" {
		t.Fatalf("ticket = %#v", ticket)
	}
}
