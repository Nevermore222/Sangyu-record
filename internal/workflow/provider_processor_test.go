package workflow

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/providerjobs"
	"github.com/nevermore222/sangyu-record/internal/providers"
)

type recordingJobSubmitter struct {
	input providerjobs.SubmitInput
	job   providerjobs.Job
}

func (s *recordingJobSubmitter) Submit(_ context.Context, input providerjobs.SubmitInput) (providerjobs.Job, error) {
	s.input = input
	return s.job, nil
}

func TestProviderProcessorSubmitsLinkedJobAndWaits(t *testing.T) {
	jobID := uuid.New()
	submitter := &recordingJobSubmitter{job: providerjobs.Job{ID: jobID}}
	processor := NewProviderProcessor(
		submitter,
		providers.KindMedia,
		providers.TaskAudioTranscription,
		func(_ context.Context, _ NodePayload) (json.RawMessage, []string, error) {
			return json.RawMessage(`{"language":"zh-CN"}`), []string{"https://objects.example/audio.wav"}, nil
		},
		"https://api.example/",
	)
	payload := NodePayload{RunID: uuid.New(), ProjectID: uuid.New(), Node: NodeTranscribe}
	before := time.Now().UTC().Add(9 * time.Minute)

	result, err := processor.Process(context.Background(), payload)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsWaiting() || result.ProviderJobID != jobID {
		t.Fatalf("result = %#v", result)
	}
	if submitter.input.ProjectID != payload.ProjectID || submitter.input.WorkflowRunID != payload.RunID || submitter.input.WorkflowNode != string(payload.Node) {
		t.Fatalf("submit linkage = %#v", submitter.input)
	}
	if submitter.input.CallbackBaseURL != "https://api.example" || len(submitter.input.ResourceURLs) != 1 {
		t.Fatalf("submit transport = %#v", submitter.input)
	}
	if submitter.input.Deadline.Before(before) {
		t.Fatalf("deadline = %s", submitter.input.Deadline)
	}
}
