package book

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var ErrInvalidPDF = errors.New("PDF engine returned invalid data")

type PDFEngine interface {
	Render(context.Context, string) ([]byte, error)
}

type ObjectStore interface {
	Put(context.Context, string, string, []byte, string) error
}

type ArtifactRepository interface {
	NextVersion(context.Context, uuid.UUID, string) (int, error)
	Save(context.Context, Artifact) error
}

type Service struct {
	engine PDFEngine
	store  ObjectStore
	repo   ArtifactRepository
	bucket string
}

func NewService(engine PDFEngine, store ObjectStore, repo ArtifactRepository, bucket string) *Service {
	return &Service{engine: engine, store: store, repo: repo, bucket: bucket}
}

func (s *Service) Render(ctx context.Context, projectID, runID uuid.UUID, manuscript Manuscript) (Artifact, error) {
	html, err := RenderHTML(manuscript)
	if err != nil {
		return Artifact{}, err
	}
	pdf, err := s.engine.Render(ctx, html)
	if err != nil {
		return Artifact{}, err
	}
	if len(pdf) == 0 || !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		return Artifact{}, ErrInvalidPDF
	}
	version, err := s.repo.NextVersion(ctx, projectID, "pdf")
	if err != nil {
		return Artifact{}, err
	}
	artifact := Artifact{
		ID: uuid.New(), ProjectID: projectID, WorkflowRunID: runID,
		Version: version, Kind: "pdf",
		ObjectKey:   fmt.Sprintf("projects/%s/artifacts/%d/memoir.pdf", projectID, version),
		ContentType: "application/pdf", SizeBytes: int64(len(pdf)), CreatedAt: time.Now().UTC(),
	}
	if err := s.store.Put(ctx, s.bucket, artifact.ObjectKey, pdf, artifact.ContentType); err != nil {
		return Artifact{}, err
	}
	if err := s.repo.Save(ctx, artifact); err != nil {
		return Artifact{}, err
	}
	return artifact, nil
}
