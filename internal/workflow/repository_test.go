package workflow

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/platform"
	"github.com/nevermore222/sangyu-record/internal/projects"
	"github.com/nevermore222/sangyu-record/internal/testdb"
)

func TestPostgresRepositoryCreatesAndAdvancesRun(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := platform.OpenPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	testdb.Serialize(t, pool)
	if _, err := pool.Exec(ctx, "TRUNCATE workflow_nodes, workflow_runs, assets, collection_plan_items, projects CASCADE"); err != nil {
		t.Fatal(err)
	}
	project, err := projects.NewService(
		projects.NewPostgresRepository(pool), projects.DeterministicPlanner{},
	).Create(ctx, projects.CreateInput{
		DisplayName: "测试老人", BirthYear: 1950, BirthPlace: "南京",
		LongTermResidence: "南京", TargetEdition: "brief",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"audio", "photo"} {
		_, err := pool.Exec(ctx, `
			INSERT INTO assets (id, project_id, kind, filename, content_type, size_bytes, object_key, sha256, state, uploaded_at)
			VALUES ($1, $2, $3, $4, $5, 10, $6, $7, 'uploaded', now())`,
			uuid.New(), project.ID, kind, kind+".bin", map[string]string{"audio": "audio/wav", "photo": "image/jpeg"}[kind],
			"projects/test/"+kind+".bin", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	repo := NewPostgresRepository(pool)
	run, err := repo.CreateRun(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Nodes) != len(NodeSequence) {
		t.Fatalf("nodes = %d, want %d", len(run.Nodes), len(NodeSequence))
	}
	loaded, err := repo.LatestRun(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	for index, node := range loaded.Nodes {
		if node.Name != NodeSequence[index] {
			t.Fatalf("node %d = %s, want %s", index, node.Name, NodeSequence[index])
		}
	}
	payload := NodePayload{RunID: run.ID, ProjectID: project.ID, Node: NodeTranscribe}
	claimed, err := repo.ClaimNode(ctx, payload)
	if err != nil || !claimed {
		t.Fatalf("claimed = %v, err = %v", claimed, err)
	}
	next, err := repo.SucceedNode(ctx, payload, json.RawMessage(`{"ok":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || next.Node != NodeUnderstandPhoto {
		t.Fatalf("next = %#v", next)
	}
	claimed, err = repo.ClaimNode(ctx, payload)
	if err != nil || claimed {
		t.Fatalf("duplicate claimed = %v, err = %v", claimed, err)
	}
}
