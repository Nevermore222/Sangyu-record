package visitanalysis

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/platform"
	"github.com/nevermore222/sangyu-record/internal/projects"
	"github.com/nevermore222/sangyu-record/internal/testdb"
	"github.com/nevermore222/sangyu-record/internal/visits"
	"github.com/nevermore222/sangyu-record/internal/workflow"
)

func TestPostgresRepositoryPersistsAnalysisAndPlanUpdates(t *testing.T) {
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
	if _, err := pool.Exec(ctx, "TRUNCATE staff CASCADE"); err != nil {
		t.Fatal(err)
	}

	staffID := uuid.New()
	if _, err := pool.Exec(ctx, `
		INSERT INTO staff (id, wechat_openid, display_name, state)
		VALUES ($1, $2, 'Analysis Tester', 'active')`, staffID, staffID.String()); err != nil {
		t.Fatal(err)
	}
	projectRepo := projects.NewPostgresRepository(pool)
	projectService := projects.NewService(projectRepo, projects.DeterministicPlanner{})
	project, err := projectService.Create(ctx, projects.CreateInput{
		OwnerStaffID: staffID, DisplayName: "Analysis Project", BirthYear: 1950,
		BirthPlace: "Suzhou", LongTermResidence: "Suzhou", TargetEdition: "brief",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projectService.ConfirmConsent(ctx, project.ID, staffID, projects.ConfirmConsentInput{ConfirmedBy: "elder"}); err != nil {
		t.Fatal(err)
	}
	visitService := visits.NewService(visits.NewPostgresRepository(pool), projectRepo, false)
	visit, err := visitService.Create(ctx, visits.CreateInput{
		ProjectID: project.ID, StaffID: staffID, VisitedAt: time.Now(),
		PlanItemIDs: []uuid.UUID{project.CollectionPlan[0].ID, project.CollectionPlan[1].ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO assets (
			id, project_id, visit_id, kind, source, filename, display_name,
			content_type, size_bytes, object_key, sha256, state, uploaded_at
		) VALUES ($1,$2,$3,'audio','direct','visit.wav','Visit audio',
		          'audio/wav',10,$4,$5,'uploaded',now())`,
		uuid.New(), project.ID, visit.ID, "projects/test/visit.wav",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatal(err)
	}

	repo := NewPostgresRepository(pool)
	if err := repo.PrepareSubmit(ctx, visit.ID); err != nil {
		t.Fatal(err)
	}
	run, err := workflow.NewPostgresRepository(pool).CreateRun(ctx, workflow.CreateRunInput{
		ProjectID: project.ID, VisitID: visit.ID,
		Kind: workflow.RunKindVisitAnalysis, Nodes: workflow.VisitAnalysisSequence,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.AttachRun(ctx, visit.ID, run.ID); err != nil {
		t.Fatal(err)
	}
	assessment := json.RawMessage(`{
		"complete":false,
		"covered_items":[{"plan_item_id":"` + project.CollectionPlan[0].ID.String() + `","evidence_refs":["audio#1-5"]}],
		"gaps":[{"plan_item_id":"` + project.CollectionPlan[1].ID.String() + `","reason":"missing detail"}]
	}`)
	followup := json.RawMessage(`{
		"summary":"The visit covered childhood and needs education detail.",
		"questions":[{"plan_item_id":"` + project.CollectionPlan[1].ID.String() + `","question":"Who was your first teacher?"}]
	}`)
	if _, err := pool.Exec(ctx, `
		UPDATE workflow_nodes SET output=CASE node_name
			WHEN 'visit_assess_material' THEN $2::jsonb
			WHEN 'visit_plan_followup' THEN $3::jsonb
			ELSE output END
		WHERE run_id=$1`, run.ID, assessment, followup); err != nil {
		t.Fatal(err)
	}
	payload := workflow.NodePayload{
		RunID: run.ID, ProjectID: project.ID, VisitID: visit.ID,
		Kind: workflow.RunKindVisitAnalysis, Node: workflow.NodeVisitPersistAnalysis,
	}
	first, err := repo.Persist(ctx, payload)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.Persist(ctx, payload)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || len(first.CoveredItems) != 1 || len(first.Gaps) != 1 || len(first.FollowupQuestions) != 1 {
		t.Fatalf("first = %#v, second = %#v", first, second)
	}

	var coveredStatus, gapStatus, gapReason, visitState, projectState string
	if err := pool.QueryRow(ctx, `SELECT status FROM collection_plan_items WHERE id=$1`, project.CollectionPlan[0].ID).Scan(&coveredStatus); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status, gap_reason FROM collection_plan_items WHERE id=$1`, project.CollectionPlan[1].ID).Scan(&gapStatus, &gapReason); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT state FROM visits WHERE id=$1`, visit.ID).Scan(&visitState); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT state FROM projects WHERE id=$1`, project.ID).Scan(&projectState); err != nil {
		t.Fatal(err)
	}
	if coveredStatus != "collected" || gapStatus != "insufficient" || gapReason != "missing detail" || visitState != "completed" || projectState != "needs_material" {
		t.Fatalf("states = %s, %s, %q, %s, %s", coveredStatus, gapStatus, gapReason, visitState, projectState)
	}

	input, err := repo.BuildProviderInput(ctx, payload)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(input), project.CollectionPlan[0].ID.String()) || !strings.Contains(string(input), "visit_assess_material") {
		t.Fatalf("provider input = %s", input)
	}
}
