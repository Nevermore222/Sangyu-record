package projects

import (
	"context"
	"os"
	"testing"

	"github.com/nevermore222/sangyu-record/internal/platform"
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
	defer pool.Close()
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
}
