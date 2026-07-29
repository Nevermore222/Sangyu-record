package visits

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	Create(context.Context, Visit, bool) (Visit, error)
	Get(context.Context, uuid.UUID, uuid.UUID, bool) (Visit, error)
	List(context.Context, uuid.UUID, uuid.UUID, bool) ([]Visit, error)
	Update(context.Context, Visit, bool) (Visit, error)
}

type ConsentChecker interface {
	HasConsent(context.Context, uuid.UUID) (bool, error)
}

type Service struct {
	repo         Repository
	consents     ConsentChecker
	allowUnowned bool
}

func NewService(repo Repository, consents ConsentChecker, allowUnowned bool) *Service {
	return &Service{repo: repo, consents: consents, allowUnowned: allowUnowned}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Visit, error) {
	if input.ProjectID == uuid.Nil || input.StaffID == uuid.Nil {
		return Visit{}, fmt.Errorf("%w: project and staff IDs are required", ErrValidation)
	}
	hasConsent, err := s.consents.HasConsent(ctx, input.ProjectID)
	if err != nil {
		return Visit{}, err
	}
	if !hasConsent {
		return Visit{}, ErrConsentRequired
	}
	if input.VisitedAt.IsZero() {
		input.VisitedAt = time.Now().UTC()
	}
	now := time.Now().UTC()
	return s.repo.Create(ctx, Visit{
		ID: uuid.New(), ProjectID: input.ProjectID, StaffID: input.StaffID,
		VisitedAt: input.VisitedAt.UTC(), Location: strings.TrimSpace(input.Location),
		Notes: strings.TrimSpace(input.Notes), State: StateDraft,
		PlanItemIDs: uniqueIDs(input.PlanItemIDs), CreatedAt: now, UpdatedAt: now,
	}, s.allowUnowned)
}

func (s *Service) Get(ctx context.Context, id, staffID uuid.UUID) (Visit, error) {
	if id == uuid.Nil || staffID == uuid.Nil {
		return Visit{}, fmt.Errorf("%w: visit and staff IDs are required", ErrValidation)
	}
	return s.repo.Get(ctx, id, staffID, s.allowUnowned)
}

func (s *Service) Authorize(ctx context.Context, id, staffID uuid.UUID) error {
	_, err := s.Get(ctx, id, staffID)
	return err
}

func (s *Service) List(ctx context.Context, projectID, staffID uuid.UUID) ([]Visit, error) {
	if projectID == uuid.Nil || staffID == uuid.Nil {
		return nil, fmt.Errorf("%w: project and staff IDs are required", ErrValidation)
	}
	return s.repo.List(ctx, projectID, staffID, s.allowUnowned)
}

func (s *Service) Update(ctx context.Context, id, staffID uuid.UUID, input UpdateInput) (Visit, error) {
	value, err := s.Get(ctx, id, staffID)
	if err != nil {
		return Visit{}, err
	}
	if value.State != StateDraft {
		return Visit{}, ErrInvalidState
	}
	if input.VisitedAt != nil {
		if input.VisitedAt.IsZero() {
			return Visit{}, fmt.Errorf("%w: visited_at cannot be empty", ErrValidation)
		}
		value.VisitedAt = input.VisitedAt.UTC()
	}
	if input.Location != nil {
		value.Location = strings.TrimSpace(*input.Location)
	}
	if input.Notes != nil {
		value.Notes = strings.TrimSpace(*input.Notes)
	}
	if input.PlanItemIDs != nil {
		value.PlanItemIDs = uniqueIDs(*input.PlanItemIDs)
	}
	value.UpdatedAt = time.Now().UTC()
	return s.repo.Update(ctx, value, s.allowUnowned)
}

func uniqueIDs(values []uuid.UUID) []uuid.UUID {
	result := make([]uuid.UUID, 0, len(values))
	seen := make(map[uuid.UUID]struct{}, len(values))
	for _, value := range values {
		if value == uuid.Nil {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
