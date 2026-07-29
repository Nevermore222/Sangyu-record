package projects

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type memoryRepository struct {
	projects map[uuid.UUID]ProjectDetail
	consents map[string]Consent
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{
		projects: make(map[uuid.UUID]ProjectDetail),
		consents: make(map[string]Consent),
	}
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

func (r *memoryRepository) GetOwned(ctx context.Context, id, _ uuid.UUID, _ bool) (ProjectDetail, error) {
	return r.Get(ctx, id)
}

func (r *memoryRepository) List(_ context.Context, input ListInput) (Page, error) {
	page := Page{Items: make([]ProjectSummary, 0)}
	for _, detail := range r.projects {
		if detail.OwnerStaffID != input.OwnerStaffID && !(input.IncludeUnowned && detail.OwnerStaffID == uuid.Nil) {
			continue
		}
		page.Items = append(page.Items, ProjectSummary{
			ID: detail.ID, OwnerStaffID: detail.OwnerStaffID, DisplayName: detail.DisplayName,
			BirthYear: detail.BirthYear, BirthPlace: detail.BirthPlace,
			LongTermResidence: detail.LongTermResidence, PrimaryOccupation: detail.PrimaryOccupation,
			TargetEdition: detail.TargetEdition, State: detail.State, UpdatedAt: detail.UpdatedAt,
		})
	}
	return page, nil
}

func (r *memoryRepository) Dashboard(ctx context.Context, ownerID uuid.UUID, includeUnowned bool) (Dashboard, error) {
	page, err := r.List(ctx, ListInput{OwnerStaffID: ownerID, IncludeUnowned: includeUnowned})
	return Dashboard{Recent: page.Items, Actionable: page.Items}, err
}

func (r *memoryRepository) UpsertConsent(_ context.Context, value Consent) (Consent, error) {
	key := value.ProjectID.String() + ":" + value.ConfirmedBy
	if existing, ok := r.consents[key]; ok {
		return existing, nil
	}
	r.consents[key] = value
	return value, nil
}

func (r *memoryRepository) HasConsent(_ context.Context, projectID uuid.UUID) (bool, error) {
	for _, value := range r.consents {
		if value.ProjectID == projectID {
			return true, nil
		}
	}
	return false, nil
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

func TestConsentIsIdempotentPerConfirmation(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(repo, DeterministicPlanner{})
	projectID := uuid.New()
	staffID := uuid.New()
	repo.projects[projectID] = ProjectDetail{Project: Project{ID: projectID, OwnerStaffID: staffID}}

	first, err := service.ConfirmConsent(context.Background(), projectID, staffID, ConfirmConsentInput{ConfirmedBy: "elder"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.ConfirmConsent(context.Background(), projectID, staffID, ConfirmConsentInput{ConfirmedBy: "elder"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatal("duplicate consent record")
	}
}

func TestConsentRejectsUnknownConfirmer(t *testing.T) {
	service := NewService(newMemoryRepository(), DeterministicPlanner{})
	_, err := service.ConfirmConsent(context.Background(), uuid.New(), uuid.New(), ConfirmConsentInput{ConfirmedBy: "staff"})
	if !errors.Is(err, ErrValidation) {
		t.Fatalf("err = %v, want ErrValidation", err)
	}
}
