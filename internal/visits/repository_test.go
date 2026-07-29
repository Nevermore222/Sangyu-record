package visits

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/platform"
	"github.com/nevermore222/sangyu-record/internal/projects"
	"github.com/nevermore222/sangyu-record/internal/testdb"
)

func TestPostgresRepositoryVisitLifecycle(t *testing.T) {
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
		VALUES ($1, $2, 'Visit Tester', 'active')`, staffID, staffID.String()); err != nil {
		t.Fatal(err)
	}
	projectRepo := projects.NewPostgresRepository(pool)
	projectService := projects.NewService(projectRepo, projects.DeterministicPlanner{})
	project, err := projectService.Create(ctx, projects.CreateInput{
		OwnerStaffID: staffID, DisplayName: "Visit Project", BirthYear: 1950,
		BirthPlace: "Suzhou", LongTermResidence: "Suzhou", TargetEdition: "brief",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := projectService.ConfirmConsent(ctx, project.ID, staffID, projects.ConfirmConsentInput{ConfirmedBy: "elder"}); err != nil {
		t.Fatal(err)
	}

	service := NewService(NewPostgresRepository(pool), projectRepo, false)
	first, err := service.Create(ctx, CreateInput{
		ProjectID: project.ID, StaffID: staffID, VisitedAt: time.Now(),
		Location: "Care Home", PlanItemIDs: []uuid.UUID{project.CollectionPlan[0].ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.Create(ctx, CreateInput{
		ProjectID: project.ID, StaffID: staffID, VisitedAt: time.Now(),
		PlanItemIDs: []uuid.UUID{project.CollectionPlan[1].ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Sequence != 1 || second.Sequence != 2 {
		t.Fatalf("sequences = %d, %d", first.Sequence, second.Sequence)
	}
	listed, err := service.List(ctx, project.ID, staffID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0].Sequence != 2 || len(listed[1].PlanItemIDs) != 1 {
		t.Fatalf("listed = %#v", listed)
	}

	if _, err := pool.Exec(ctx, "UPDATE visits SET state='submitted' WHERE id=$1", first.ID); err != nil {
		t.Fatal(err)
	}
	notes := "cannot edit"
	if _, err := service.Update(ctx, first.ID, staffID, UpdateInput{Notes: &notes}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("err = %v, want ErrInvalidState", err)
	}
	if _, err := service.Get(ctx, second.ID, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other owner err = %v, want ErrNotFound", err)
	}
}
