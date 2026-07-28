package projects

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type memoryRepository struct {
	projects map[uuid.UUID]ProjectDetail
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{projects: make(map[uuid.UUID]ProjectDetail)}
}

func (r *memoryRepository) Create(_ context.Context, detail ProjectDetail) error {
	r.projects[detail.ID] = detail
	return nil
}

func (r *memoryRepository) Get(_ context.Context, id uuid.UUID) (ProjectDetail, error) {
	detail, ok := r.projects[id]
	if !ok {
		return ProjectDetail{}, ErrNotFound
	}
	return detail, nil
}

func TestCreateGeneratesFirstCollectionPlan(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(repo, DeterministicPlanner{})

	created, err := service.Create(context.Background(), CreateInput{
		DisplayName:       "林奶奶",
		BirthYear:         1948,
		BirthPlace:        "江苏苏州",
		LongTermResidence: "江苏苏州",
		PrimaryOccupation: "纺织工人",
		TargetEdition:     "standard",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.State != StateCollecting {
		t.Fatalf("state = %s, want %s", created.State, StateCollecting)
	}
	if len(created.CollectionPlan) != 7 {
		t.Fatalf("plan items = %d, want 7", len(created.CollectionPlan))
	}
	if created.CollectionPlan[0].Status != PlanPending {
		t.Fatalf("first item status = %s, want %s", created.CollectionPlan[0].Status, PlanPending)
	}
	if created.CollectionPlan[6].Category != "occupation" {
		t.Fatalf("last category = %s, want occupation", created.CollectionPlan[6].Category)
	}
}

func TestCreateRejectsInvalidBirthYear(t *testing.T) {
	service := NewService(newMemoryRepository(), DeterministicPlanner{})
	_, err := service.Create(context.Background(), CreateInput{
		DisplayName:       "测试老人",
		BirthYear:         1899,
		BirthPlace:        "苏州",
		LongTermResidence: "苏州",
		TargetEdition:     "brief",
	})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}
