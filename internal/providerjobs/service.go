package providerjobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/providers"
)

type SubmitInput struct {
	ProjectID       uuid.UUID
	WorkflowRunID   uuid.UUID
	WorkflowNode    string
	ProviderKind    providers.Kind
	TaskType        providers.TaskType
	Input           json.RawMessage
	ResourceURLs    []string
	CallbackBaseURL string
	Deadline        time.Time
}

type Service struct {
	repo     Repository
	rawStore RawStore
	registry providers.Registry
	now      func() time.Time
}

func NewService(repo Repository, rawStore RawStore, registry providers.Registry, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, rawStore: rawStore, registry: registry, now: now}
}

func (s *Service) Submit(ctx context.Context, input SubmitInput) (Job, error) {
	requestID := uuid.New()
	persistedInput, err := json.Marshal(struct {
		TaskInput       json.RawMessage `json:"task_input"`
		ResourceURLs    []string        `json:"resource_urls"`
		CallbackBaseURL string          `json:"callback_base_url"`
	}{TaskInput: input.Input, ResourceURLs: input.ResourceURLs, CallbackBaseURL: input.CallbackBaseURL})
	if err != nil {
		return Job{}, err
	}
	job, _, err := s.repo.CreateOrGet(ctx, CreateInput{
		RequestID: requestID, ProjectID: input.ProjectID, WorkflowRunID: input.WorkflowRunID,
		WorkflowNode: input.WorkflowNode, ProviderKind: input.ProviderKind, TaskType: input.TaskType,
		IdempotencyKey: input.WorkflowRunID.String() + ":" + input.WorkflowNode,
		Input:          persistedInput, Deadline: input.Deadline,
	})
	if err != nil {
		return Job{}, err
	}
	if job.State != providers.StatePendingSubmission && !(job.State == providers.StateRetryableFailed && job.ProviderJobID == "") {
		return job, nil
	}
	return s.submitRemote(ctx, job)
}

func (s *Service) submitRemote(ctx context.Context, job Job) (Job, error) {
	var persisted struct {
		TaskInput       json.RawMessage `json:"task_input"`
		ResourceURLs    []string        `json:"resource_urls"`
		CallbackBaseURL string          `json:"callback_base_url"`
	}
	if err := json.Unmarshal(job.Input, &persisted); err != nil {
		return Job{}, err
	}
	provider, err := s.registry.For(job.ProviderKind)
	if err != nil {
		return Job{}, err
	}
	attempt, err := s.repo.StartAttempt(ctx, job.ID, "submit", job.State)
	if err != nil {
		return Job{}, err
	}
	started := s.now()
	ref, remoteErr := provider.Submit(ctx, providers.SubmitRequest{
		RequestID: job.RequestID.String(), IdempotencyKey: job.IdempotencyKey, ContractVersion: "1.0",
		TaskType: job.TaskType, InputSchemaVersion: "1.0", OutputSchemaVersion: "1.0", Input: persisted.TaskInput,
		ResourceURLs: persisted.ResourceURLs,
		CallbackURL:  strings.TrimRight(persisted.CallbackBaseURL, "/") + "/v1/provider-callbacks/" + string(job.ProviderKind) + "/" + job.ID.String(),
		Deadline:     job.Deadline,
	})
	attempt.ElapsedMS = s.now().Sub(started).Milliseconds()
	if remoteErr != nil {
		state, code := stateForError(remoteErr)
		attempt.State, attempt.ErrorCode = state, code
		if err := s.repo.FinishAttempt(ctx, attempt); err != nil {
			return Job{}, err
		}
		if err := s.repo.ApplySnapshot(ctx, job.ID, providers.Snapshot{State: state, ErrorCode: code, ErrorMessage: remoteErr.Error()}, ""); err != nil {
			return Job{}, err
		}
		return s.repo.Get(ctx, job.ID)
	}
	if len(ref.Raw) > 0 {
		key, err := s.rawStore.Put(ctx, job, attempt.Attempt, ref.Raw)
		if err != nil {
			return Job{}, err
		}
		attempt.RawResponseObjectKey = key
	}
	if err := validateSubmitResponse(ref); err != nil {
		attempt.State, attempt.ErrorCode = providers.StatePermanentlyFailed, "invalid_provider_response"
		if finishErr := s.repo.FinishAttempt(ctx, attempt); finishErr != nil {
			return Job{}, finishErr
		}
		if applyErr := s.repo.ApplySnapshot(ctx, job.ID, providers.Snapshot{
			ProviderJobID: ref.ProviderJobID,
			State:         providers.StatePermanentlyFailed,
			ErrorCode:     "invalid_provider_response",
			ErrorMessage:  err.Error(),
		}, attempt.RawResponseObjectKey); applyErr != nil {
			return Job{}, applyErr
		}
		return s.repo.Get(ctx, job.ID)
	}
	attempt.State = ref.State
	if err := s.repo.FinishAttempt(ctx, attempt); err != nil {
		return Job{}, err
	}
	if err := s.repo.MarkSubmitted(ctx, job.ID, ref); err != nil {
		return Job{}, err
	}
	return s.repo.Get(ctx, job.ID)
}

func (s *Service) Refresh(ctx context.Context, jobID uuid.UUID) (Job, error) {
	job, err := s.repo.Get(ctx, jobID)
	if err != nil || job.State.Terminal() {
		return job, err
	}
	if !job.Deadline.After(s.now()) {
		if err := s.repo.ApplySnapshot(ctx, job.ID, providers.Snapshot{State: providers.StateTimedOut, ErrorCode: "provider_deadline_exceeded"}, ""); err != nil {
			return Job{}, err
		}
		return s.repo.Get(ctx, job.ID)
	}
	if job.ProviderJobID == "" {
		return s.submitRemote(ctx, job)
	}
	provider, err := s.registry.For(job.ProviderKind)
	if err != nil {
		return Job{}, err
	}
	attempt, err := s.repo.StartAttempt(ctx, job.ID, "status", job.State)
	if err != nil {
		return Job{}, err
	}
	started := s.now()
	snapshot, remoteErr := provider.Status(ctx, job.ProviderJobID)
	attempt.ElapsedMS = s.now().Sub(started).Milliseconds()
	if remoteErr != nil {
		state, code := stateForError(remoteErr)
		snapshot = providers.Snapshot{
			RequestID: job.RequestID.String(), ProviderJobID: job.ProviderJobID,
			State: state, ErrorCode: code, ErrorMessage: remoteErr.Error(),
		}
	}
	return s.applyAttemptSnapshot(ctx, job, attempt, snapshot)
}

func (s *Service) ApplyCallback(ctx context.Context, jobID uuid.UUID, snapshot providers.Snapshot) error {
	job, err := s.repo.Get(ctx, jobID)
	if err != nil {
		return err
	}
	if err := validateSnapshotIdentity(job, snapshot); err != nil {
		return err
	}
	if err := validateSnapshotEnvelope(snapshot); err != nil {
		return err
	}
	if job.State == snapshot.State {
		return nil
	}
	if job.State.Terminal() {
		return ErrTerminalConflict
	}
	attempt, err := s.repo.StartAttempt(ctx, job.ID, "callback", job.State)
	if err != nil {
		return err
	}
	_, err = s.applyAttemptSnapshot(ctx, job, attempt, snapshot)
	return err
}

func (s *Service) applyAttemptSnapshot(ctx context.Context, job Job, attempt Attempt, snapshot providers.Snapshot) (Job, error) {
	rawKey := ""
	if len(snapshot.Raw) > 0 {
		key, err := s.rawStore.Put(ctx, job, attempt.Attempt, snapshot.Raw)
		if err != nil {
			return Job{}, err
		}
		rawKey, attempt.RawResponseObjectKey = key, key
	}
	if err := validateSnapshotIdentity(job, snapshot); err != nil {
		attempt.State, attempt.ErrorCode = providers.StatePermanentlyFailed, "invalid_provider_identity"
		if finishErr := s.repo.FinishAttempt(ctx, attempt); finishErr != nil {
			return Job{}, finishErr
		}
		return Job{}, err
	}
	if err := validateSnapshotEnvelope(snapshot); err != nil {
		attempt.State, attempt.ErrorCode = providers.StatePermanentlyFailed, "invalid_provider_response"
		if finishErr := s.repo.FinishAttempt(ctx, attempt); finishErr != nil {
			return Job{}, finishErr
		}
		if applyErr := s.repo.ApplySnapshot(ctx, job.ID, providers.Snapshot{
			ProviderJobID: snapshot.ProviderJobID,
			State:         providers.StatePermanentlyFailed,
			ErrorCode:     "invalid_provider_response",
			ErrorMessage:  err.Error(),
		}, rawKey); applyErr != nil {
			return Job{}, applyErr
		}
		return s.repo.Get(ctx, job.ID)
	}
	if snapshot.State == providers.StateSucceeded {
		normalized, err := providers.Normalize(job.TaskType, snapshot.Output)
		if err != nil {
			snapshot.State, snapshot.ErrorCode, snapshot.ErrorMessage = providers.StatePermanentlyFailed, "invalid_provider_output", err.Error()
			snapshot.Output = nil
		} else {
			snapshot.Output = normalized
		}
	}
	attempt.State, attempt.ErrorCode = snapshot.State, snapshot.ErrorCode
	if err := s.repo.FinishAttempt(ctx, attempt); err != nil {
		return Job{}, err
	}
	if err := s.repo.ApplySnapshot(ctx, job.ID, snapshot, rawKey); err != nil {
		return Job{}, err
	}
	return s.repo.Get(ctx, job.ID)
}

func validateSnapshotIdentity(job Job, snapshot providers.Snapshot) error {
	if snapshot.RequestID == "" || snapshot.RequestID != job.RequestID.String() {
		return errors.New("provider snapshot request ID does not match job")
	}
	if snapshot.ProviderJobID == "" {
		return errors.New("provider snapshot job ID is required")
	}
	if job.ProviderJobID != "" && snapshot.ProviderJobID != job.ProviderJobID {
		return errors.New("provider snapshot job ID does not match")
	}
	return nil
}

func validateSubmitResponse(ref providers.JobRef) error {
	if ref.ProviderJobID == "" {
		return errors.New("provider submit response job ID is required")
	}
	if ref.State != providers.StateSubmitted && ref.State != providers.StateProcessing {
		return errors.New("provider submit response must be submitted or processing")
	}
	return nil
}

func validateSnapshotEnvelope(snapshot providers.Snapshot) error {
	switch snapshot.State {
	case providers.StateSubmitted, providers.StateProcessing, providers.StateSucceeded:
		return nil
	case providers.StateCancelled:
		return nil
	case providers.StateRetryableFailed, providers.StatePermanentlyFailed, providers.StateTimedOut:
		if snapshot.ErrorCode == "" {
			return errors.New("provider failure snapshot error code is required")
		}
		return nil
	default:
		return errors.New("provider snapshot state is invalid")
	}
}

func (s *Service) ConsumeTerminal(ctx context.Context, jobID uuid.UUID) (Outcome, bool, error) {
	return s.repo.ConsumeTerminal(ctx, jobID)
}

func (s *Service) FindUnconsumedByWorkflowNode(
	ctx context.Context,
	projectID uuid.UUID,
	runID uuid.UUID,
	node string,
) (Job, error) {
	return s.repo.FindUnconsumedByWorkflowNode(ctx, projectID, runID, node)
}

func (s *Service) ListUnconsumedDue(ctx context.Context, before time.Time, cursor DueCursor, limit int) ([]Job, error) {
	return s.repo.ListUnconsumedDue(ctx, before, cursor, limit)
}

func (s *Service) PeekTerminal(ctx context.Context, jobID uuid.UUID) (Outcome, bool, error) {
	return s.repo.PeekTerminal(ctx, jobID)
}

func (s *Service) MarkConsumed(ctx context.Context, jobID uuid.UUID) error {
	return s.repo.MarkConsumed(ctx, jobID)
}

func (s *Service) Get(ctx context.Context, jobID uuid.UUID) (Job, error) {
	return s.repo.Get(ctx, jobID)
}

func stateForError(err error) (providers.State, string) {
	var remote *providers.RemoteError
	if errors.As(err, &remote) {
		code := remote.Code
		if code == "" {
			code = fmt.Sprintf("provider_http_%d", remote.StatusCode)
		}
		if remote.StatusCode == http.StatusTooManyRequests || remote.StatusCode >= 500 {
			return providers.StateRetryableFailed, code
		}
		return providers.StatePermanentlyFailed, code
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return providers.StateTimedOut, "provider_deadline_exceeded"
	}
	return providers.StateRetryableFailed, "provider_transport_error"
}
