package workflow

import (
	"context"
	"encoding/json"
	"errors"
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
	if _, err := pool.Exec(ctx, "TRUNCATE workflow_nodes, workflow_runs, assets, collection_plan_items, projects, staff CASCADE"); err != nil {
		t.Fatal(err)
	}
	ownerID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO staff (id, wechat_openid, display_name, state)
		VALUES ($1, $2, 'Project Owner', 'active')`, ownerID, ownerID.String()); err != nil {
		t.Fatal(err)
	}
	project, err := projects.NewService(
		projects.NewPostgresRepository(pool), projects.DeterministicPlanner{},
	).Create(ctx, projects.CreateInput{
		OwnerStaffID: ownerID,
		DisplayName:  "测试老人", BirthYear: 1950, BirthPlace: "南京",
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
	run, err := repo.CreateRun(ctx, CreateRunInput{
		ProjectID: project.ID,
		Kind:      RunKindBook,
		Nodes:     NodeSequence,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(run.Nodes) != len(NodeSequence) {
		t.Fatalf("nodes = %d, want %d", len(run.Nodes), len(NodeSequence))
	}
	loaded, err := repo.LatestRun(ctx, project.ID, project.OwnerStaffID, false)
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
	next, err = repo.SucceedNode(ctx, payload, json.RawMessage(`{"ok":true}`))
	if err != nil || next == nil || next.Node != NodeUnderstandPhoto {
		t.Fatalf("idempotent next = %#v, err = %v", next, err)
	}
	claimed, err = repo.ClaimNode(ctx, payload)
	if err != nil || claimed {
		t.Fatalf("duplicate claimed = %v, err = %v", claimed, err)
	}

	staffID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO staff (id, wechat_openid, display_name, state)
		VALUES ($1, $2, 'Workflow Tester', 'active')`, staffID, staffID.String()); err != nil {
		t.Fatal(err)
	}
	visitID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO visits (
			id, project_id, sequence, staff_id, visited_at, state, created_at, updated_at
		) VALUES ($1, $2, 1, $3, now(), 'draft', now(), now())`,
		visitID, project.ID, staffID); err != nil {
		t.Fatal(err)
	}
	visitRun, err := repo.CreateRun(ctx, CreateRunInput{
		ProjectID: project.ID,
		VisitID:   visitID,
		Kind:      RunKindVisitAnalysis,
		Nodes:     VisitAnalysisSequence,
	})
	if err != nil {
		t.Fatal(err)
	}
	if visitRun.Kind != RunKindVisitAnalysis || visitRun.VisitID != visitID || len(visitRun.Nodes) != len(VisitAnalysisSequence) {
		t.Fatalf("visit run = %#v", visitRun)
	}
	for position, node := range visitRun.Nodes {
		if node.Name != VisitAnalysisSequence[position] || node.Position != position {
			t.Fatalf("visit node %d = %#v", position, node)
		}
	}
	visitPayload := NodePayload{
		RunID: visitRun.ID, ProjectID: project.ID, VisitID: visitID,
		Kind: RunKindVisitAnalysis, Node: NodeVisitTranscribe,
	}
	claimed, err = repo.ClaimNode(ctx, visitPayload)
	if err != nil || !claimed {
		t.Fatalf("visit claimed = %t, err = %v", claimed, err)
	}
	visitNext, err := repo.SucceedNode(ctx, visitPayload, json.RawMessage(`{"ok":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if visitNext == nil || visitNext.Node != NodeVisitUnderstandPhoto || visitNext.VisitID != visitID || visitNext.Kind != RunKindVisitAnalysis {
		t.Fatalf("visit next = %#v", visitNext)
	}
}

func TestPostgresRepositoryGuardsAndDeduplicatesFinalization(t *testing.T) {
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
	if _, err := pool.Exec(ctx, "TRUNCATE workflow_nodes, workflow_runs, assets, visits, consents, collection_plan_items, projects, staff CASCADE"); err != nil {
		t.Fatal(err)
	}

	ownerID, otherID := uuid.New(), uuid.New()
	for _, staffID := range []uuid.UUID{ownerID, otherID} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO staff (id, wechat_openid, display_name, state)
			VALUES ($1, $2, 'Finalization Tester', 'active')`, staffID, staffID.String()); err != nil {
			t.Fatal(err)
		}
	}
	project, err := projects.NewService(
		projects.NewPostgresRepository(pool), projects.DeterministicPlanner{},
	).Create(ctx, projects.CreateInput{
		OwnerStaffID: ownerID, DisplayName: "Finalization Project", BirthYear: 1948,
		BirthPlace: "Suzhou", LongTermResidence: "Suzhou", TargetEdition: "brief",
	})
	if err != nil {
		t.Fatal(err)
	}
	repo := NewPostgresRepository(pool)
	request := FinalizeBookRequest{ProjectID: project.ID, StaffID: ownerID, Nodes: NodeSequence}

	if _, _, err := repo.FinalizeBook(ctx, request); !errors.Is(err, ErrConsentRequired) {
		t.Fatalf("without consent err = %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO consents (id, project_id, confirmed_by, confirmation_method, staff_id, confirmed_at)
		VALUES ($1, $2, 'elder', 'onsite', $3, now())`, uuid.New(), project.ID, ownerID); err != nil {
		t.Fatal(err)
	}
	visitID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO visits (id, project_id, sequence, staff_id, visited_at, state)
		VALUES ($1, $2, 1, $3, now(), 'draft')`, visitID, project.ID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.FinalizeBook(ctx, request); !errors.Is(err, ErrDraftVisitExists) {
		t.Fatalf("with draft visit err = %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE visits SET state='completed' WHERE id=$1", visitID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.FinalizeBook(ctx, request); !errors.Is(err, ErrInsufficientAssets) {
		t.Fatalf("without assets err = %v", err)
	}
	for _, kind := range []string{"audio", "photo"} {
		contentType := map[string]string{"audio": "audio/wav", "photo": "image/jpeg"}[kind]
		if _, err := pool.Exec(ctx, `
			INSERT INTO assets (id, project_id, kind, filename, content_type, size_bytes, object_key, sha256, state, uploaded_at)
			VALUES ($1, $2, $3, $4, $5, 10, $6, $7, 'uploaded', now())`,
			uuid.New(), project.ID, kind, kind+".bin", contentType, "projects/finalize/"+kind,
			"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
			t.Fatal(err)
		}
	}

	first, created, err := repo.FinalizeBook(ctx, request)
	if err != nil || !created {
		t.Fatalf("first run = %#v, created = %t, err = %v", first, created, err)
	}
	second, created, err := repo.FinalizeBook(ctx, request)
	if err != nil || created || second.ID != first.ID {
		t.Fatalf("second run = %#v, created = %t, err = %v", second, created, err)
	}
	request.StaffID = otherID
	if _, _, err := repo.FinalizeBook(ctx, request); !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf("other owner err = %v", err)
	}
}
