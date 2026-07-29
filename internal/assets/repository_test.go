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
}
