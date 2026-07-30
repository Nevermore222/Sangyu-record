package visitanalysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/visits"
	"github.com/nevermore222/sangyu-record/internal/workflow"
)

type fakeVisitAccess struct {
	visit visits.Visit
	err   error
}

func (a fakeVisitAccess) Get(_ context.Context, _, _ uuid.UUID) (visits.Visit, error) {
	return a.visit, a.err
}

type fakeAnalysisRepository struct {
	prepareErr    error
	attachedRunID uuid.UUID
	retryPayload  workflow.NodePayload
	reverted      bool
	analysis      Analysis
}

func (r *fakeAnalysisRepository) PrepareSubmit(_ context.Context, _ uuid.UUID) error {
	return r.prepareErr
}

func (r *fakeAnalysisRepository) AttachRun(_ context.Context, _ uuid.UUID, runID uuid.UUID) error {
	r.attachedRunID = runID
	return nil
}

func (r *fakeAnalysisRepository) RevertSubmit(_ context.Context, _ uuid.UUID) error {
	r.reverted = true
	return nil
}

func (r *fakeAnalysisRepository) RetryPayload(_ context.Context, _ uuid.UUID) (workflow.NodePayload, error) {
	return r.retryPayload, nil
}

func (r *fakeAnalysisRepository) GetByVisit(_ context.Context, _ uuid.UUID) (Analysis, error) {
	return r.analysis, nil
}

type fakeWorkflowStarter struct {
	input workflow.CreateRunInput
	err   error
}

func (s *fakeWorkflowStarter) StartRun(_ context.Context, input workflow.CreateRunInput) (workflow.Run, error) {
	s.input = input
	run := workflow.Run{
		ID: uuid.New(), ProjectID: input.ProjectID, VisitID: input.VisitID,
		Kind: input.Kind, State: workflow.NodeQueued, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	return run, s.err
}

type fakeNodeQueue struct {
	payloads []workflow.NodePayload
}

func (q *fakeNodeQueue) EnqueueNode(_ context.Context, payload workflow.NodePayload) error {
	q.payloads = append(q.payloads, payload)
	return nil
}

func TestSubmitStartsVisitAnalysisSequence(t *testing.T) {
	visit := visits.Visit{ID: uuid.New(), ProjectID: uuid.New(), StaffID: uuid.New(), State: visits.StateDraft}
	repo := &fakeAnalysisRepository{}
	starter := &fakeWorkflowStarter{}
	service := NewService(repo, fakeVisitAccess{visit: visit}, starter, &fakeNodeQueue{})

	run, err := service.Submit(context.Background(), visit.ID, visit.StaffID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Kind != workflow.RunKindVisitAnalysis || starter.input.VisitID != visit.ID || len(starter.input.Nodes) != 5 {
		t.Fatalf("run = %#v, input = %#v", run, starter.input)
	}
	if repo.attachedRunID != run.ID {
		t.Fatalf("attached run = %s, want %s", repo.attachedRunID, run.ID)
	}
}

func TestSubmitRejectsInsufficientAssets(t *testing.T) {
	visit := visits.Visit{ID: uuid.New(), ProjectID: uuid.New(), StaffID: uuid.New(), State: visits.StateDraft}
	service := NewService(
		&fakeAnalysisRepository{prepareErr: ErrInsufficientAssets},
		fakeVisitAccess{visit: visit}, &fakeWorkflowStarter{}, &fakeNodeQueue{},
	)
	if _, err := service.Submit(context.Background(), visit.ID, visit.StaffID); !errors.Is(err, ErrInsufficientAssets) {
		t.Fatalf("err = %v, want ErrInsufficientAssets", err)
	}
}

func TestRetryQueuesFailedNode(t *testing.T) {
	visit := visits.Visit{ID: uuid.New(), ProjectID: uuid.New(), StaffID: uuid.New(), State: visits.StateFailed}
	payload := workflow.NodePayload{
		RunID: uuid.New(), ProjectID: visit.ProjectID, VisitID: visit.ID,
		Kind: workflow.RunKindVisitAnalysis, Node: workflow.NodeVisitAssessMaterial,
	}
	queue := &fakeNodeQueue{}
	service := NewService(
		&fakeAnalysisRepository{retryPayload: payload},
		fakeVisitAccess{visit: visit}, &fakeWorkflowStarter{}, queue,
	)
	if _, err := service.Retry(context.Background(), visit.ID, visit.StaffID); err != nil {
		t.Fatal(err)
	}
	if len(queue.payloads) != 1 || queue.payloads[0] != payload {
		t.Fatalf("queued = %#v", queue.payloads)
	}
}
