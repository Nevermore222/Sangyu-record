package book

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/workflow"
)

type ManuscriptRepository interface {
	LoadManuscript(context.Context, uuid.UUID) (Manuscript, error)
}

type WorkflowProcessor struct {
	repo     ManuscriptRepository
	renderer *Service
}

func NewWorkflowProcessor(repo ManuscriptRepository, renderer *Service) *WorkflowProcessor {
	return &WorkflowProcessor{repo: repo, renderer: renderer}
}

func (p *WorkflowProcessor) Process(ctx context.Context, payload workflow.NodePayload) (workflow.ProcessResult, error) {
	manuscript, err := p.repo.LoadManuscript(ctx, payload.RunID)
	if err != nil {
		return workflow.ProcessResult{}, err
	}
	artifact, err := p.renderer.Render(ctx, payload.ProjectID, payload.RunID, manuscript)
	if err != nil {
		return workflow.ProcessResult{}, err
	}
	encoded, err := json.Marshal(artifact)
	return workflow.Completed(encoded), err
}
