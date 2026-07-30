package visitanalysis

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/providers"
	"github.com/nevermore222/sangyu-record/internal/workflow"
)

type fakePersistenceRepository struct {
	analysis Analysis
	calls    int
}

func (r *fakePersistenceRepository) Persist(_ context.Context, _ workflow.NodePayload) (Analysis, error) {
	r.calls++
	return r.analysis, nil
}

func TestPersistProcessorReturnsAnalysisOutput(t *testing.T) {
	repo := &fakePersistenceRepository{analysis: Analysis{
		ID: uuid.New(), VisitID: uuid.New(), WorkflowRunID: uuid.New(), Summary: "Visit summary",
		CoveredItems: []providers.CoveredItem{{PlanItemID: uuid.NewString(), EvidenceRefs: []string{"audio#1-2"}}},
	}}
	result, err := NewPersistProcessor(repo).Process(context.Background(), workflow.NodePayload{
		RunID: repo.analysis.WorkflowRunID, VisitID: repo.analysis.VisitID,
		Node: workflow.NodeVisitPersistAnalysis,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsWaiting() || !json.Valid(result.Output) || repo.calls != 1 {
		t.Fatalf("result = %#v, calls = %d", result, repo.calls)
	}
}
