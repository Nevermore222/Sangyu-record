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
	repo    Repository
	planner Planner
}

func NewService(repo Repository, planner Planner) *Service {
	return &Service{repo: repo, planner: planner}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (ProjectDetail, error) {
	if err := validateCreateInput(input); err != nil {
		return ProjectDetail{}, err
	}

	now := time.Now().UTC()
	projectID := uuid.New()
	detail := ProjectDetail{Project: Project{
		ID:                projectID,
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
