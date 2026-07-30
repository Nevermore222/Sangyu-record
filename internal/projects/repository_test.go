package projects

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevermore222/sangyu-record/internal/platform"
	"github.com/nevermore222/sangyu-record/internal/testdb"
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
	if _, err := pool.Exec(ctx, "TRUNCATE collection_plan_items, projects CASCADE"); err != nil {
		t.Fatal(err)
	}

	repo := NewPostgresRepository(pool)
	service := NewService(repo, DeterministicPlanner{})
	created, err := service.Create(ctx, CreateInput{
		DisplayName:       "周爷爷",
		BirthYear:         1950,
		BirthPlace:        "浙江宁波",
		LongTermResidence: "上海",
		PrimaryOccupation: "教师",
		TargetEdition:     "brief",
	})
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := service.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.DisplayName != "周爷爷" || len(loaded.CollectionPlan) != 7 {
		t.Fatalf("loaded = %#v", loaded)
	}
	for index, item := range loaded.CollectionPlan {
		if item.Position != index {
			t.Fatalf("plan position = %d, want %d", item.Position, index)
		}
	}

	gapReason := "missing a specific date"
	if _, err := pool.Exec(ctx, `
		UPDATE collection_plan_items SET status='insufficient', gap_reason=$2
		WHERE id=$1`, created.CollectionPlan[0].ID, gapReason); err != nil {
		t.Fatal(err)
	}
	loaded, err = service.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CollectionPlan[0].GapReason != gapReason {
		t.Fatalf("gap reason = %q, want %q", loaded.CollectionPlan[0].GapReason, gapReason)
	}
}

func TestPostgresRepositoryListsOnlyOwnedProjectsWithCursor(t *testing.T) {
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
	if _, err := pool.Exec(ctx, "TRUNCATE collection_plan_items, projects, staff CASCADE"); err != nil {
		t.Fatal(err)
	}

	ownerID := uuid.New()
	otherOwnerID := uuid.New()
	for _, id := range []uuid.UUID{ownerID, otherOwnerID} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO staff (id, wechat_openid, display_name, state)
			VALUES ($1, $2, 'Repository Owner', 'active')`, id, id.String()); err != nil {
			t.Fatal(err)
		}
	}

	repo := NewPostgresRepository(pool)
	service := NewService(repo, DeterministicPlanner{})
	for index := 0; index < 3; index++ {
		created, err := service.Create(ctx, CreateInput{
			OwnerStaffID: ownerID, DisplayName: "Owned Project",
			BirthYear: 1950 + index, BirthPlace: "Suzhou", LongTermResidence: "Shanghai",
			TargetEdition: "brief",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, "UPDATE projects SET updated_at=$2 WHERE id=$1", created.ID, time.Now().UTC().Add(time.Duration(index)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.Create(ctx, CreateInput{
		OwnerStaffID: otherOwnerID, DisplayName: "Other Project", BirthYear: 1940,
		BirthPlace: "Nanjing", LongTermResidence: "Nanjing", TargetEdition: "brief",
	}); err != nil {
		t.Fatal(err)
	}

	first, err := repo.List(ctx, ListInput{OwnerStaffID: ownerID, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 2 || first.NextCursor == "" {
		t.Fatalf("first page = %#v", first)
	}
	second, err := repo.List(ctx, ListInput{OwnerStaffID: ownerID, Limit: 2, Cursor: first.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 1 || second.NextCursor != "" {
		t.Fatalf("second page = %#v", second)
	}
	for _, item := range append(first.Items, second.Items...) {
		if item.OwnerStaffID != ownerID {
			t.Fatalf("project owner = %s, want %s", item.OwnerStaffID, ownerID)
		}
	}
	projectID := first.Items[0].ID
	firstConsent, err := service.ConfirmConsent(ctx, projectID, ownerID, ConfirmConsentInput{ConfirmedBy: "elder"})
	if err != nil {
		t.Fatal(err)
	}
	secondConsent, err := service.ConfirmConsent(ctx, projectID, ownerID, ConfirmConsentInput{ConfirmedBy: "elder"})
	if err != nil {
		t.Fatal(err)
	}
	if firstConsent.ID != secondConsent.ID {
		t.Fatalf("consent IDs = %s and %s", firstConsent.ID, secondConsent.ID)
	}
	hasConsent, err := repo.HasConsent(ctx, projectID)
	if err != nil || !hasConsent {
		t.Fatalf("has consent = %t, err = %v", hasConsent, err)
	}
	detail, err := repo.GetOwned(ctx, projectID, ownerID, false)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Consent == nil || detail.Consent.ID != firstConsent.ID {
		t.Fatalf("detail consent = %#v", detail.Consent)
	}
}
