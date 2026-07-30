package visitanalysis

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/assets"
	"github.com/nevermore222/sangyu-record/internal/providers"
	"github.com/nevermore222/sangyu-record/internal/workflow"
)

type VisitURLReader interface {
	URLsByVisit(context.Context, uuid.UUID, assets.Kind) ([]string, error)
}

type ProviderContextReader interface {
	BuildProviderInput(context.Context, workflow.NodePayload) (json.RawMessage, error)
}

func ProviderProcessors(
	submitter workflow.JobSubmitter,
	contextReader ProviderContextReader,
	sourceReader VisitURLReader,
	callbackBaseURL string,
) map[workflow.NodeName]workflow.Processor {
	type spec struct {
		kind      providers.Kind
		task      providers.TaskType
		assetKind assets.Kind
	}
	specs := map[workflow.NodeName]spec{
		workflow.NodeVisitTranscribe:      {kind: providers.KindMedia, task: providers.TaskAudioTranscription, assetKind: assets.KindAudio},
		workflow.NodeVisitUnderstandPhoto: {kind: providers.KindMedia, task: providers.TaskPhotoUnderstanding, assetKind: assets.KindPhoto},
		workflow.NodeVisitAssessMaterial:  {kind: providers.KindAgent, task: providers.TaskMaterialAssessment},
		workflow.NodeVisitPlanFollowup:    {kind: providers.KindAgent, task: providers.TaskFollowupPlan},
	}
	result := make(map[workflow.NodeName]workflow.Processor, len(specs))
	for node, nodeSpec := range specs {
		nodeSpec := nodeSpec
		result[node] = workflow.NewProviderProcessor(
			submitter,
			nodeSpec.kind,
			nodeSpec.task,
			func(ctx context.Context, payload workflow.NodePayload) (json.RawMessage, []string, error) {
				input, err := contextReader.BuildProviderInput(ctx, payload)
				if err != nil {
					return nil, nil, err
				}
				if nodeSpec.assetKind == "" {
					return input, []string{}, nil
				}
				resources, err := sourceReader.URLsByVisit(ctx, payload.VisitID, nodeSpec.assetKind)
				return input, resources, err
			},
			callbackBaseURL,
		)
	}
	return result
}

type PersistenceRepository interface {
	Persist(context.Context, workflow.NodePayload) (Analysis, error)
}

type PersistProcessor struct {
	repo PersistenceRepository
}

func NewPersistProcessor(repo PersistenceRepository) *PersistProcessor {
	return &PersistProcessor{repo: repo}
}

func (p *PersistProcessor) Process(ctx context.Context, payload workflow.NodePayload) (workflow.ProcessResult, error) {
	analysis, err := p.repo.Persist(ctx, payload)
	if err != nil {
		return workflow.ProcessResult{}, err
	}
	output, err := json.Marshal(analysis)
	if err != nil {
		return workflow.ProcessResult{}, err
	}
	return workflow.Completed(output), nil
}
