package workflow

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/assets"
	"github.com/nevermore222/sangyu-record/internal/providers"
)

type AssetURLReader interface {
	URLs(context.Context, uuid.UUID, assets.Kind) ([]string, error)
}

type providerNodeSpec struct {
	kind      providers.Kind
	task      providers.TaskType
	assetKind assets.Kind
}

func ProviderProcessors(submitter JobSubmitter, sourceReader AssetURLReader, callbackBaseURL string) map[NodeName]Processor {
	specs := map[NodeName]providerNodeSpec{
		NodeTranscribe:           {kind: providers.KindMedia, task: providers.TaskAudioTranscription, assetKind: assets.KindAudio},
		NodeUnderstandPhoto:      {kind: providers.KindMedia, task: providers.TaskPhotoUnderstanding, assetKind: assets.KindPhoto},
		NodeBuildMemory:          {kind: providers.KindAgent, task: providers.TaskTimelineBuilder},
		NodeRetrieveSharedMemory: {kind: providers.KindKnowledge, task: providers.TaskSharedMemoryRetrieval},
		NodePlanBook:             {kind: providers.KindAgent, task: providers.TaskChapterPlanner},
		NodeWriteBook:            {kind: providers.KindAgent, task: providers.TaskChapterWriter},
	}
	processors := make(map[NodeName]Processor, len(specs))
	for node, spec := range specs {
		processors[node] = NewProviderProcessor(submitter, spec.kind, spec.task, func(ctx context.Context, payload NodePayload) (json.RawMessage, []string, error) {
			input, err := json.Marshal(map[string]string{
				"project_id": payload.ProjectID.String(), "workflow_run_id": payload.RunID.String(), "workflow_node": string(payload.Node),
			})
			if err != nil {
				return nil, nil, err
			}
			if spec.assetKind == "" {
				return input, []string{}, nil
			}
			resources, err := sourceReader.URLs(ctx, payload.ProjectID, spec.assetKind)
			return input, resources, err
		}, callbackBaseURL)
	}
	return processors
}
