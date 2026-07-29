package workflow

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/assets"
	"github.com/nevermore222/sangyu-record/internal/providerjobs"
	"github.com/nevermore222/sangyu-record/internal/providers"
)

type mappingSubmitter struct {
	inputs []providerjobs.SubmitInput
}

func (s *mappingSubmitter) Submit(_ context.Context, input providerjobs.SubmitInput) (providerjobs.Job, error) {
	s.inputs = append(s.inputs, input)
	return providerjobs.Job{ID: uuid.New()}, nil
}

type mappingAssetReader struct {
	kinds []assets.Kind
}

func (r *mappingAssetReader) URLs(_ context.Context, _ uuid.UUID, kind assets.Kind) ([]string, error) {
	r.kinds = append(r.kinds, kind)
	return []string{"https://objects.example/" + string(kind)}, nil
}

func TestProviderProcessorsMapKindsTasksAndResourceAccess(t *testing.T) {
	submitter := &mappingSubmitter{}
	reader := &mappingAssetReader{}
	processors := ProviderProcessors(submitter, reader, "https://api.example")
	payload := NodePayload{RunID: uuid.New(), ProjectID: uuid.New()}
	want := []struct {
		node      NodeName
		kind      providers.Kind
		task      providers.TaskType
		resources int
	}{
		{NodeTranscribe, providers.KindMedia, providers.TaskAudioTranscription, 1},
		{NodeUnderstandPhoto, providers.KindMedia, providers.TaskPhotoUnderstanding, 1},
		{NodeBuildMemory, providers.KindAgent, providers.TaskTimelineBuilder, 0},
		{NodeRetrieveSharedMemory, providers.KindKnowledge, providers.TaskSharedMemoryRetrieval, 0},
		{NodePlanBook, providers.KindAgent, providers.TaskChapterPlanner, 0},
		{NodeWriteBook, providers.KindAgent, providers.TaskChapterWriter, 0},
	}

	for index, expected := range want {
		payload.Node = expected.node
		if _, err := processors[expected.node].Process(context.Background(), payload); err != nil {
			t.Fatal(err)
		}
		input := submitter.inputs[index]
		if input.ProviderKind != expected.kind || input.TaskType != expected.task || len(input.ResourceURLs) != expected.resources {
			t.Fatalf("%s submission = %#v", expected.node, input)
		}
	}
	if len(reader.kinds) != 2 || reader.kinds[0] != assets.KindAudio || reader.kinds[1] != assets.KindPhoto {
		t.Fatalf("asset reader kinds = %#v", reader.kinds)
	}
}
