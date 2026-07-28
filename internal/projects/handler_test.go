package projects

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
