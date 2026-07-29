package assets

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/platform"
	"github.com/nevermore222/sangyu-record/internal/projects"
	"github.com/nevermore222/sangyu-record/internal/testdb"
	"github.com/nevermore222/sangyu-record/internal/visits"
)

func TestPostgresRepositoryRoundTrip(t *testing.T) {
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
	if _, err := pool.Exec(ctx, "TRUNCATE assets, collection_plan_items, projects CASCADE"); err != nil {
		t.Fatal(err)
	}

	project, err := projects.NewService(
		projects.NewPostgresRepository(pool), projects.DeterministicPlanner{},
	).Create(ctx, projects.CreateInput{
		DisplayName: "测试老人", BirthYear: 1955, BirthPlace: "南京",
		LongTermResidence: "南京", TargetEdition: "brief",
	})
	if err != nil {
		t.Fatal(err)
	}

	repo := NewPostgresRepository(pool)
	asset := Asset{
		ID: uuid.New(), ProjectID: project.ID, Kind: KindAudio,
		Filename: "interview.wav", ContentType: "audio/wav", SizeBytes: 100,
		ObjectKey: "projects/test/source/interview.wav", State: StatePendingUpload,
		CreatedAt: time.Now().UTC(),
	}
	if err := repo.Create(ctx, asset); err != nil {
		t.Fatal(err)
	}
	hash := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	completed, err := repo.MarkUploaded(ctx, asset.ID, hash, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if completed.State != StateUploaded || completed.SHA256 != hash || completed.ObjectKey != asset.ObjectKey {
		t.Fatalf("completed = %#v", completed)
	}
	uploaded, err := repo.ListUploadedByKind(ctx, project.ID, KindAudio)
	if err != nil {
		t.Fatal(err)
	}
	if len(uploaded) != 1 || uploaded[0].ID != asset.ID {
		t.Fatalf("uploaded audio = %#v", uploaded)
	}
	photos, err := repo.ListUploadedByKind(ctx, project.ID, KindPhoto)
	if err != nil || len(photos) != 0 {
		t.Fatalf("uploaded photos = %#v, err = %v", photos, err)
	}
}

func TestPostgresRepositoryVisitAssetAssociations(t *testing.T) {
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
		VALUES ($1, $2, 'Asset Tester', 'active')`, staffID, staffID.String()); err != nil {
		t.Fatal(err)
	}
	projectRepo := projects.NewPostgresRepository(pool)
	projectService := projects.NewService(projectRepo, projects.DeterministicPlanner{})
	project, err := projectService.Create(ctx, projects.CreateInput{
		OwnerStaffID: staffID, DisplayName: "Asset Project", BirthYear: 1950,
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
		PlanItemIDs: []uuid.UUID{project.CollectionPlan[0].ID},
	})
	if err != nil {
		t.Fatal(err)
	}

	repo := NewPostgresRepository(pool)
	asset := Asset{
		ID: uuid.New(), ProjectID: project.ID, VisitID: &visit.ID,
		Kind: KindPhoto, Source: SourceCamera, Filename: "portrait.jpg", DisplayName: "Portrait",
		ContentType: "image/jpeg", SizeBytes: 100, ObjectKey: "projects/test/source/portrait.jpg",
		State: StatePendingUpload, PlanItemIDs: []uuid.UUID{project.CollectionPlan[0].ID}, CreatedAt: time.Now().UTC(),
	}
	if err := repo.Create(ctx, asset); err != nil {
		t.Fatal(err)
	}
	items, err := repo.ListByVisit(ctx, visit.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Source != SourceCamera || len(items[0].PlanItemIDs) != 1 {
		t.Fatalf("items = %#v", items)
	}
	deleted, err := repo.DeletePending(ctx, asset.ID)
	if err != nil || deleted.ID != asset.ID {
		t.Fatalf("deleted = %#v, err = %v", deleted, err)
	}
}
