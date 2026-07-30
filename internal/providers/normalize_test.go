package providers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestNormalizeTranscriptionRequiresEvidenceSource(t *testing.T) {
	_, err := Normalize(TaskAudioTranscription, json.RawMessage(`{
		"segments":[{"start_seconds":0,"end_seconds":4,"text":"我小时候住在苏州。","source_ref":""}]
	}`))
	if !errors.Is(err, ErrInvalidOutput) {
		t.Fatalf("error = %v, want ErrInvalidOutput", err)
	}
}

func TestNormalizeSharedMemoryKeepsKnowledgeCitation(t *testing.T) {
	input := json.RawMessage(`{
		"entries":[{"reference_id":"K-1978-SZ-1","text":"背景资料","source":"苏州地方志","year_from":1978,"year_to":1982,"region":"江苏苏州","confidence":"high","license":"internal-use"}]
	}`)
	normalized, err := Normalize(TaskSharedMemoryRetrieval, input)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(normalized) {
		t.Fatal("normalized result is not JSON")
	}
}

func TestNormalizeFoundationTaskOutputs(t *testing.T) {
	tests := []struct {
		task providersTask
		raw  string
	}{
		{TaskPhotoUnderstanding, `{"description":"一张合影","source_ref":"photo-1"}`},
		{TaskTimelineBuilder, `{"memories":[{"event":"参加工作","evidence_refs":["audio-1#0-2"]}]}`},
		{TaskChapterPlanner, `{"title":"岁月留声","chapters":[{"title":"第一章"}]}`},
		{TaskChapterWriter, `{"title":"岁月留声","chapters":[{"title":"第一章","paragraphs":[{"text":"正文","evidence_refs":["audio-1#0-2"]}]}]}`},
	}
	for _, test := range tests {
		t.Run(string(test.task), func(t *testing.T) {
			if _, err := Normalize(TaskType(test.task), json.RawMessage(test.raw)); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestNormalizeMaterialAssessment(t *testing.T) {
	raw := json.RawMessage(`{
		"complete":false,
		"covered_items":[{"plan_item_id":"p1","evidence_refs":["audio#1-5"]}],
		"gaps":[{"plan_item_id":"p2","reason":"missing detail"}]
	}`)
	normalized, err := Normalize(TaskMaterialAssessment, raw)
	if err != nil || !json.Valid(normalized) {
		t.Fatalf("normalized=%s err=%v", normalized, err)
	}
}

func TestNormalizeFollowupPlanRejectsEmptyQuestions(t *testing.T) {
	_, err := Normalize(TaskFollowupPlan, json.RawMessage(`{"summary":"More detail is needed","questions":[]}`))
	if !errors.Is(err, ErrInvalidOutput) {
		t.Fatalf("err = %v, want ErrInvalidOutput", err)
	}
}

type providersTask = TaskType

type noOpProvider struct{}

func (noOpProvider) Submit(context.Context, SubmitRequest) (JobRef, error) { return JobRef{}, nil }
func (noOpProvider) Status(context.Context, string) (Snapshot, error)      { return Snapshot{}, nil }
func (noOpProvider) Cancel(context.Context, string) error                  { return nil }

func TestRegistryKeepsProviderKindsSeparate(t *testing.T) {
	registry := Registry{Media: noOpProvider{}, Knowledge: noOpProvider{}, Agent: noOpProvider{}}
	for _, kind := range []Kind{KindMedia, KindKnowledge, KindAgent} {
		if _, err := registry.For(kind); err != nil {
			t.Fatalf("kind %s: %v", kind, err)
		}
	}
	if _, err := (Registry{}).For(KindAgent); !errors.Is(err, ErrKindNotConfigured) {
		t.Fatalf("error = %v, want ErrKindNotConfigured", err)
	}
}
