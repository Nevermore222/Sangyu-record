package visits

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type memoryRepository struct {
	items map[uuid.UUID]Visit
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{items: make(map[uuid.UUID]Visit)}
}

func (r *memoryRepository) Create(_ context.Context, value Visit, _ bool) (Visit, error) {
	value.Sequence = 1
	r.items[value.ID] = value
	return value, nil
}

func (r *memoryRepository) Get(_ context.Context, id, _ uuid.UUID, _ bool) (Visit, error) {
	value, ok := r.items[id]
	if !ok {
		return Visit{}, ErrNotFound
	}
	return value, nil
}

func (r *memoryRepository) List(_ context.Context, projectID, _ uuid.UUID, _ bool) ([]Visit, error) {
	items := make([]Visit, 0)
	for _, value := range r.items {
		if value.ProjectID == projectID {
			items = append(items, value)
		}
	}
	return items, nil
}

func (r *memoryRepository) Update(_ context.Context, value Visit, _ bool) (Visit, error) {
	current, ok := r.items[value.ID]
	if !ok {
		return Visit{}, ErrNotFound
	}
	if current.State != StateDraft {
		return Visit{}, ErrInvalidState
	}
	r.items[value.ID] = value
	return value, nil
}

type consentChecker struct {
	allowed bool
}

func (c consentChecker) HasConsent(_ context.Context, _ uuid.UUID) (bool, error) {
	return c.allowed, nil
}

func TestCreateVisitRequiresConsent(t *testing.T) {
	service := NewService(newMemoryRepository(), consentChecker{}, false)
	_, err := service.Create(context.Background(), CreateInput{
		ProjectID: uuid.New(), StaffID: uuid.New(), VisitedAt: time.Now(),
	})
	if !errors.Is(err, ErrConsentRequired) {
		t.Fatalf("err = %v, want ErrConsentRequired", err)
	}
}

func TestCreateAndUpdateDraftVisit(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(repo, consentChecker{allowed: true}, false)
	created, err := service.Create(context.Background(), CreateInput{
		ProjectID: uuid.New(), StaffID: uuid.New(), VisitedAt: time.Now(), Location: "Care Home",
	})
	if err != nil {
		t.Fatal(err)
	}
	updatedLocation := "City Park"
	updated, err := service.Update(context.Background(), created.ID, created.StaffID, UpdateInput{Location: &updatedLocation})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Sequence != 1 || updated.Location != updatedLocation {
		t.Fatalf("updated = %#v", updated)
	}
}

func TestUpdateRejectsSubmittedVisit(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(repo, consentChecker{allowed: true}, false)
	created, err := service.Create(context.Background(), CreateInput{
		ProjectID: uuid.New(), StaffID: uuid.New(), VisitedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	value := repo.items[created.ID]
	value.State = StateSubmitted
	repo.items[created.ID] = value
	notes := "too late"
	if _, err := service.Update(context.Background(), created.ID, created.StaffID, UpdateInput{Notes: &notes}); !errors.Is(err, ErrInvalidState) {
		t.Fatalf("err = %v, want ErrInvalidState", err)
	}
}
