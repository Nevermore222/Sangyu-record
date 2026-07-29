package providerjobs

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/providers"
)

func TestCallbackRejectsExpiredTimestamp(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	verifier := NewCallbackVerifier([]byte("test-secret"), 5*time.Minute, func() time.Time { return now })
	body := []byte(`{"request_id":"r","provider_job_id":"p","state":"processing"}`)
	timestamp := "2026-07-30T11:50:00Z"
	signature := SignCallback([]byte("test-secret"), timestamp, body)
	if !errors.Is(verifier.Verify(timestamp, signature, body), ErrCallbackExpired) {
		t.Fatal("expired callback was accepted")
	}
}

type callbackApplication struct {
	job        Job
	applyCalls int
}

func (a *callbackApplication) Get(_ context.Context, _ uuid.UUID) (Job, error) { return a.job, nil }
func (a *callbackApplication) ApplyCallback(_ context.Context, _ uuid.UUID, _ providers.Snapshot) error {
	a.applyCalls++
	return nil
}

type callbackQueue struct{ calls int }

func (q *callbackQueue) EnqueueProviderPoll(_ context.Context, _ uuid.UUID, _ time.Duration) error {
	q.calls++
	return nil
}

func TestCallbackHandlerRejectsInvalidSignatureBeforeApplying(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	app := &callbackApplication{job: Job{ProviderKind: providers.KindMedia}}
	handler := NewCallbackHandler(app, &callbackQueue{}, NewCallbackVerifier([]byte("secret"), 5*time.Minute, func() time.Time { return now }))
	router := chi.NewRouter()
	handler.Register(router)
	body := []byte(`{"request_id":"r","provider_job_id":"p","state":"processing"}`)
	request := httptest.NewRequest(http.MethodPost, "/v1/provider-callbacks/media/"+uuid.NewString(), bytes.NewReader(body))
	request.Header.Set("X-Sangyu-Timestamp", now.Format(time.RFC3339))
	request.Header.Set("X-Sangyu-Signature", "invalid")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || app.applyCalls != 0 {
		t.Fatalf("status=%d applyCalls=%d", response.Code, app.applyCalls)
	}
}

func TestCallbackHandlerAppliesAndQueuesValidSnapshot(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	jobID := uuid.New()
	app := &callbackApplication{job: Job{ID: jobID, ProviderKind: providers.KindMedia}}
	queue := &callbackQueue{}
	secret := []byte("secret")
	handler := NewCallbackHandler(app, queue, NewCallbackVerifier(secret, 5*time.Minute, func() time.Time { return now }))
	router := chi.NewRouter()
	handler.Register(router)
	body := []byte(`{"request_id":"r","provider_job_id":"p","state":"processing"}`)
	timestamp := now.Format(time.RFC3339)
	request := httptest.NewRequest(http.MethodPost, "/v1/provider-callbacks/media/"+jobID.String(), bytes.NewReader(body))
	request.Header.Set("X-Sangyu-Timestamp", timestamp)
	request.Header.Set("X-Sangyu-Signature", SignCallback(secret, timestamp, body))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusAccepted || app.applyCalls != 1 || queue.calls != 1 {
		t.Fatalf("status=%d applyCalls=%d queueCalls=%d", response.Code, app.applyCalls, queue.calls)
	}
}

func TestCallbackAcceptsCurrentValidSignature(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	verifier := NewCallbackVerifier([]byte("test-secret"), 5*time.Minute, func() time.Time { return now })
	body := []byte(`{"request_id":"r","provider_job_id":"p","state":"processing"}`)
	timestamp := now.Format(time.RFC3339)
	if err := verifier.Verify(timestamp, SignCallback([]byte("test-secret"), timestamp, body), body); err != nil {
		t.Fatal(err)
	}
}
