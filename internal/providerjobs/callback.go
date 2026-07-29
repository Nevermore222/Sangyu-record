package providerjobs

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/providers"
)

var (
	ErrCallbackExpired   = errors.New("provider callback timestamp is expired")
	ErrCallbackSignature = errors.New("provider callback signature is invalid")
)

type CallbackVerifier struct {
	secret []byte
	maxAge time.Duration
	now    func() time.Time
}

func NewCallbackVerifier(secret []byte, maxAge time.Duration, now func() time.Time) *CallbackVerifier {
	if now == nil {
		now = time.Now
	}
	return &CallbackVerifier{secret: append([]byte(nil), secret...), maxAge: maxAge, now: now}
}

func SignCallback(secret []byte, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func (v *CallbackVerifier) Verify(timestamp, signature string, body []byte) error {
	sentAt, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return ErrCallbackExpired
	}
	age := v.now().Sub(sentAt)
	if age < 0 {
		age = -age
	}
	if age > v.maxAge {
		return ErrCallbackExpired
	}
	expected, err := hex.DecodeString(SignCallback(v.secret, timestamp, body))
	if err != nil {
		return ErrCallbackSignature
	}
	actual, err := hex.DecodeString(signature)
	if err != nil || !hmac.Equal(expected, actual) {
		return ErrCallbackSignature
	}
	return nil
}

type CallbackApplication interface {
	Get(context.Context, uuid.UUID) (Job, error)
	ApplyCallback(context.Context, uuid.UUID, providers.Snapshot) error
}

type PollEnqueuer interface {
	EnqueueProviderPoll(context.Context, uuid.UUID, time.Duration) error
}

type CallbackHandler struct {
	app      CallbackApplication
	queue    PollEnqueuer
	verifier *CallbackVerifier
}

func NewCallbackHandler(app CallbackApplication, queue PollEnqueuer, verifier *CallbackVerifier) *CallbackHandler {
	return &CallbackHandler{app: app, queue: queue, verifier: verifier}
}

func (h *CallbackHandler) Register(router chi.Router) {
	router.Post("/v1/provider-callbacks/{kind}/{jobID}", h.Handle)
}

func (h *CallbackHandler) Handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, (10<<20)+1))
	if err != nil || len(body) > 10<<20 {
		writeCallbackError(w, http.StatusRequestEntityTooLarge, "callback_too_large")
		return
	}
	if err := h.verifier.Verify(r.Header.Get("X-Sangyu-Timestamp"), r.Header.Get("X-Sangyu-Signature"), body); err != nil {
		writeCallbackError(w, http.StatusUnauthorized, "invalid_callback_signature")
		return
	}
	jobID, err := uuid.Parse(chi.URLParam(r, "jobID"))
	if err != nil {
		writeCallbackError(w, http.StatusUnprocessableEntity, "invalid_provider_job_id")
		return
	}
	job, err := h.app.Get(r.Context(), jobID)
	if err != nil {
		writeCallbackError(w, http.StatusNotFound, "provider_job_not_found")
		return
	}
	if providers.Kind(chi.URLParam(r, "kind")) != job.ProviderKind {
		writeCallbackError(w, http.StatusConflict, "provider_kind_mismatch")
		return
	}
	var snapshot providers.Snapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		writeCallbackError(w, http.StatusUnprocessableEntity, "invalid_callback_body")
		return
	}
	snapshot.Raw = append(json.RawMessage(nil), body...)
	if err := h.app.ApplyCallback(r.Context(), jobID, snapshot); err != nil {
		writeCallbackError(w, http.StatusConflict, "callback_not_applied")
		return
	}
	if err := h.queue.EnqueueProviderPoll(r.Context(), jobID, 0); err != nil {
		writeCallbackError(w, http.StatusInternalServerError, "callback_queue_failed")
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func writeCallbackError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": code}})
}
