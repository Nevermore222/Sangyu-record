package book

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/workflow"
)

type memoryManuscriptRepository struct {
	manuscript Manuscript
}

func (r memoryManuscriptRepository) LoadManuscript(_ context.Context, _ uuid.UUID) (Manuscript, error) {
	return r.manuscript, nil
}

func TestWorkflowProcessorRendersStoredManuscript(t *testing.T) {
	artifactRepo := &memoryArtifactRepository{}
	renderer := NewService(
		fakePDFEngine{data: []byte("%PDF-1.4\nfixture")},
		&memoryArtifactStore{}, artifactRepo, "private",
	)
	processor := NewWorkflowProcessor(memoryManuscriptRepository{manuscript: Manuscript{
		Title:    "岁月留声",
		Chapters: []Chapter{{Title: "第一章", Paragraphs: []Paragraph{{Text: "正文", EvidenceRefs: []string{"audio#1-2"}}}}},
	}}, renderer)
	payload := workflow.NodePayload{RunID: uuid.New(), ProjectID: uuid.New(), Node: workflow.NodeRenderPDF}

	result, err := processor.Process(context.Background(), payload)
	if err != nil {
		t.Fatal(err)
	}
	var artifact Artifact
	if err := json.Unmarshal(result.Output, &artifact); err != nil {
		t.Fatal(err)
	}
	if artifact.Kind != "pdf" || artifact.SizeBytes == 0 {
		t.Fatalf("artifact = %#v", artifact)
	}
}
