package book

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/platform"
)

func TestPostgresRepositoryLoadsManuscriptAndSavesArtifact(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := platform.OpenPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, "TRUNCATE artifacts, workflow_nodes, workflow_runs, assets, collection_plan_items, projects CASCADE"); err != nil {
		t.Fatal(err)
	}
	projectID, runID := uuid.New(), uuid.New()
	if _, err := pool.Exec(ctx, `INSERT INTO projects (id, display_name, birth_year, birth_place, long_term_residence, target_edition) VALUES ($1, '测试', 1950, '苏州', '苏州', 'brief')`, projectID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO workflow_runs (id, project_id, state) VALUES ($1, $2, 'running')`, runID, projectID); err != nil {
		t.Fatal(err)
	}
	output := `{"project_id":"` + projectID.String() + `","output":{"title":"岁月留声","chapters":[{"title":"第一章","paragraphs":[{"text":"正文","evidence_refs":["audio#1-2"]}]}]}}`
	if _, err := pool.Exec(ctx, `INSERT INTO workflow_nodes (id, run_id, node_name, state, output) VALUES ($1, $2, 'write_book', 'succeeded', $3)`, uuid.New(), runID, output); err != nil {
		t.Fatal(err)
	}

	repo := NewPostgresRepository(pool)
	manuscript, err := repo.LoadManuscript(ctx, runID)
	if err != nil {
		t.Fatal(err)
	}
	if manuscript.Title != "岁月留声" || len(manuscript.Chapters) != 1 {
		t.Fatalf("manuscript = %#v", manuscript)
	}
	version, err := repo.NextVersion(ctx, projectID, "pdf")
	if err != nil || version != 1 {
		t.Fatalf("version = %d, err = %v", version, err)
	}
	artifact := Artifact{ID: uuid.New(), ProjectID: projectID, WorkflowRunID: runID, Version: 1, Kind: "pdf", ObjectKey: "projects/test/memoir.pdf", ContentType: "application/pdf", SizeBytes: 10, CreatedAt: time.Now().UTC()}
	if err := repo.Save(ctx, artifact); err != nil {
		t.Fatal(err)
	}
	latest, err := repo.Latest(ctx, projectID)
	if err != nil || latest.ID != artifact.ID {
		t.Fatalf("latest = %#v, err = %v", latest, err)
	}
}
