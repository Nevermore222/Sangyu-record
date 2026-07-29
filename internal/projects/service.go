package projects

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	Create(context.Context, ProjectDetail) error
	Get(context.Context, uuid.UUID) (ProjectDetail, error)
	GetOwned(context.Context, uuid.UUID, uuid.UUID, bool) (ProjectDetail, error)
	List(context.Context, ListInput) (Page, error)
	Dashboard(context.Context, uuid.UUID, bool) (Dashboard, error)
	UpsertConsent(context.Context, Consent) (Consent, error)
	HasConsent(context.Context, uuid.UUID) (bool, error)
}

type Planner interface {
	BuildInitialPlan(CreateInput) []PlanItem
}

type DeterministicPlanner struct{}

func (DeterministicPlanner) BuildInitialPlan(input CreateInput) []PlanItem {
	items := []PlanItem{
		{Category: "childhood", Prompt: "请讲述童年生活环境、父母和印象最深的事情。", Required: true},
		{Category: "education", Prompt: "请讲述求学经历、老师同学和离开学校的过程。", Required: true},
		{Category: "work", Prompt: "请讲述第一次工作、主要工作变化和代表故事。", Required: true},
		{Category: "family", Prompt: "请讲述婚姻、子女和家庭生活中的重要时刻。", Required: true},
		{Category: "turning_points", Prompt: "请讲述改变人生方向的选择、迁居或重大事件。", Required: true},
		{Category: "photos", Prompt: "请采集童年、家庭、工作和重要物件照片。", Required: true},
	}
	if strings.TrimSpace(input.PrimaryOccupation) != "" {
		items = append(items, PlanItem{
			Category: "occupation",
			Prompt:   fmt.Sprintf("请围绕%s的工作环境、同事关系和行业变化深入采访。", strings.TrimSpace(input.PrimaryOccupation)),
			Required: true,
		})
	}
	return items
}

type Service struct {
	repo         Repository
	planner      Planner
	allowUnowned bool
}

func NewService(repo Repository, planner Planner) *Service {
	return &Service{repo: repo, planner: planner}
}

func NewServiceWithConfig(repo Repository, planner Planner, allowUnowned bool) *Service {
	return &Service{repo: repo, planner: planner, allowUnowned: allowUnowned}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (ProjectDetail, error) {
	if err := validateCreateInput(input); err != nil {
		return ProjectDetail{}, err
	}

	now := time.Now().UTC()
	projectID := uuid.New()
	detail := ProjectDetail{Project: Project{
		ID:                projectID,
		OwnerStaffID:      input.OwnerStaffID,
		DisplayName:       strings.TrimSpace(input.DisplayName),
		BirthYear:         input.BirthYear,
		BirthPlace:        strings.TrimSpace(input.BirthPlace),
		LongTermResidence: strings.TrimSpace(input.LongTermResidence),
		PrimaryOccupation: strings.TrimSpace(input.PrimaryOccupation),
		TargetEdition:     input.TargetEdition,
		State:             StateCollecting,
		CreatedAt:         now,
		UpdatedAt:         now,
	}}

	detail.CollectionPlan = s.planner.BuildInitialPlan(input)
	for index := range detail.CollectionPlan {
		detail.CollectionPlan[index].ID = uuid.New()
		detail.CollectionPlan[index].ProjectID = projectID
		detail.CollectionPlan[index].Status = PlanPending
		detail.CollectionPlan[index].Position = index
		detail.CollectionPlan[index].CreatedAt = now
	}

	if err := s.repo.Create(ctx, detail); err != nil {
		return ProjectDetail{}, err
	}
	return detail, nil
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (ProjectDetail, error) {
	return s.repo.Get(ctx, id)
}

func (s *Service) GetOwned(ctx context.Context, id, ownerStaffID uuid.UUID) (ProjectDetail, error) {
	return s.repo.GetOwned(ctx, id, ownerStaffID, s.allowUnowned)
}

func (s *Service) List(ctx context.Context, input ListInput) (Page, error) {
	if input.OwnerStaffID == uuid.Nil {
		return Page{}, fmt.Errorf("%w: owner staff ID is required", ErrValidation)
	}
	if input.Limit <= 0 {
		input.Limit = 20
	} else if input.Limit > 50 {
		input.Limit = 50
	}
	input.Query = strings.TrimSpace(input.Query)
	if input.State != "" && !validState(input.State) {
		return Page{}, fmt.Errorf("%w: unknown project state", ErrValidation)
	}
	input.IncludeUnowned = s.allowUnowned
	return s.repo.List(ctx, input)
}

func (s *Service) Dashboard(ctx context.Context, ownerStaffID uuid.UUID) (Dashboard, error) {
	if ownerStaffID == uuid.Nil {
		return Dashboard{}, fmt.Errorf("%w: owner staff ID is required", ErrValidation)
	}
	return s.repo.Dashboard(ctx, ownerStaffID, s.allowUnowned)
}

func (s *Service) ConfirmConsent(ctx context.Context, projectID, staffID uuid.UUID, input ConfirmConsentInput) (Consent, error) {
	if projectID == uuid.Nil || staffID == uuid.Nil {
		return Consent{}, fmt.Errorf("%w: project and staff IDs are required", ErrValidation)
	}
	if input.ConfirmedBy != "elder" && input.ConfirmedBy != "guardian" {
		return Consent{}, fmt.Errorf("%w: confirmed_by must be elder or guardian", ErrValidation)
	}
	if _, err := s.repo.GetOwned(ctx, projectID, staffID, s.allowUnowned); err != nil {
		return Consent{}, err
	}
	return s.repo.UpsertConsent(ctx, Consent{
		ID: uuid.New(), ProjectID: projectID, ConfirmedBy: input.ConfirmedBy,
		ConfirmationMethod: "onsite", StaffID: staffID, ConfirmedAt: time.Now().UTC(),
	})
}

func validState(state State) bool {
	switch state {
	case StateCollecting, StateProcessing, StateNeedsMaterial, StateGenerating,
		StateQualityCheck, StateException, StatePDFRendering, StateCompleted:
		return true
	default:
		return false
	}
}

func validateCreateInput(input CreateInput) error {
	if strings.TrimSpace(input.DisplayName) == "" {
		return fmt.Errorf("%w: display_name is required", ErrValidation)
	}
	if input.BirthYear < 1900 || input.BirthYear > time.Now().Year() {
		return fmt.Errorf("%w: birth_year is out of range", ErrValidation)
	}
	if strings.TrimSpace(input.BirthPlace) == "" || strings.TrimSpace(input.LongTermResidence) == "" {
		return fmt.Errorf("%w: birth_place and long_term_residence are required", ErrValidation)
	}
	switch input.TargetEdition {
	case "brief", "standard", "long":
		return nil
	default:
		return fmt.Errorf("%w: target_edition must be brief, standard, or long", ErrValidation)
	}
}
