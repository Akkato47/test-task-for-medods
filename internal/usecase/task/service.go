package task

import (
	"context"
	"fmt"
	"strings"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

const generationHorizon = 90 * 24 * time.Hour

type Service struct {
	repo         Repository
	templateRepo TemplateRepository
	now          func() time.Time
}

func NewService(repo Repository, templateRepo TemplateRepository) *Service {
	return &Service{
		repo:         repo,
		templateRepo: templateRepo,
		now:          func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) ([]taskdomain.Task, error) {
	normalized, err := validateCreateInput(input)
	if err != nil {
		return nil, err
	}

	if normalized.Period == nil {
		return s.createSingleTask(ctx, normalized)
	}

	return s.createPeriodicTasks(ctx, normalized)
}

func (s *Service) createSingleTask(ctx context.Context, input CreateInput) ([]taskdomain.Task, error) {
	now := s.now()
	model := &taskdomain.Task{
		Title:         input.Title,
		Description:   input.Description,
		Status:        input.Status,
		ScheduledDate: input.ScheduledDate,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	created, err := s.repo.Create(ctx, model)
	if err != nil {
		return nil, err
	}

	return []taskdomain.Task{*created}, nil
}

func (s *Service) createPeriodicTasks(ctx context.Context, input CreateInput) ([]taskdomain.Task, error) {
	p := input.Period
	now := s.now()

	tmpl := &taskdomain.Template{
		Title:       input.Title,
		Description: input.Description,
		Period: taskdomain.Period{
			Type:       p.Type,
			Interval:   p.Interval,
			DayOfMonth: p.DayOfMonth,
			Dates:      p.Dates,
			Parity:     p.Parity,
		},
		StartDate: input.ScheduledDate,
		EndDate:   p.EndDate,
		CreatedAt: now,
		UpdatedAt: now,
	}

	savedTmpl, err := s.templateRepo.Create(ctx, tmpl)
	if err != nil {
		return nil, err
	}

	initialTo := input.ScheduledDate.Add(generationHorizon)
	if savedTmpl.EndDate != nil && savedTmpl.EndDate.Before(initialTo) {
		initialTo = *savedTmpl.EndDate
	}

	if err := s.generateForTemplate(ctx, savedTmpl, input.ScheduledDate, initialTo, now); err != nil {
		return nil, err
	}

	return s.repo.List(ctx, &input.ScheduledDate, &initialTo)
}

func (s *Service) ensureInstancesExist(ctx context.Context, from, to time.Time) error {
	templates, err := s.templateRepo.ListActive(ctx)
	if err != nil {
		return err
	}

	now := s.now()

	for i := range templates {
		tmpl := &templates[i]

		start := from
		if tmpl.StartDate.After(start) {
			start = tmpl.StartDate
		}

		end := to
		if tmpl.EndDate != nil && tmpl.EndDate.Before(end) {
			end = *tmpl.EndDate
		}

		if start.After(end) {
			continue
		}

		if err := s.generateForTemplate(ctx, tmpl, start, end, now); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) generateForTemplate(ctx context.Context, tmpl *taskdomain.Template, from, to time.Time, now time.Time) error {
	dates := tmpl.Period.Compute(from, to, tmpl.StartDate)
	if len(dates) == 0 {
		return nil
	}

	tasks := make([]*taskdomain.Task, 0, len(dates))
	for _, d := range dates {
		scheduled := d
		tasks = append(tasks, &taskdomain.Task{
			Title:         tmpl.Title,
			Description:   tmpl.Description,
			Status:        taskdomain.StatusNew,
			TemplateID:    &tmpl.ID,
			ScheduledDate: scheduled,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
	}

	return s.repo.CreateBatchIgnoreDuplicates(ctx, tasks)
}

func (s *Service) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	return s.repo.GetByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	normalized, err := validateUpdateInput(input)
	if err != nil {
		return nil, err
	}

	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	now := s.now()

	task.Title = normalized.Title
	task.Description = normalized.Description
	task.Status = normalized.Status
	task.UpdatedAt = now

	updated, err := s.repo.Update(ctx, task)
	if err != nil {
		return nil, err
	}

	if task.TemplateID != nil && normalized.Scope != ScopeThis {
		patch := taskdomain.TaskPatch{
			Title:       normalized.Title,
			Description: normalized.Description,
			Status:      normalized.Status,
		}

		switch normalized.Scope {
		case ScopeThisAndFollowing:
			if err := s.repo.UpdateByTemplateIDFromDate(ctx, *task.TemplateID, task.ScheduledDate, patch, now); err != nil {
				return nil, err
			}

			if err := s.updateTemplate(ctx, *task.TemplateID, normalized.Title, normalized.Description, now); err != nil {
				return nil, err
			}

		case ScopeAll:
			if err := s.repo.UpdateByTemplateID(ctx, *task.TemplateID, patch, now); err != nil {
				return nil, err
			}

			if err := s.updateTemplate(ctx, *task.TemplateID, normalized.Title, normalized.Description, now); err != nil {
				return nil, err
			}
		}
	}

	return updated, nil
}

func (s *Service) updateTemplate(ctx context.Context, templateID int64, title, description string, now time.Time) error {
	tmpl, err := s.templateRepo.GetByID(ctx, templateID)
	if err != nil {
		return err
	}

	tmpl.Title = title
	tmpl.Description = description
	tmpl.UpdatedAt = now

	_, err = s.templateRepo.Update(ctx, tmpl)

	return err
}

func (s *Service) Delete(ctx context.Context, id int64, scope Scope) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	if scope == "" {
		scope = ScopeThis
	}

	if !scope.valid() {
		return fmt.Errorf("%w: invalid scope %q", ErrInvalidInput, scope)
	}

	task, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if task.TemplateID == nil || scope == ScopeThis {
		return s.repo.Delete(ctx, id)
	}

	templateID := *task.TemplateID

	switch scope {
	case ScopeThisAndFollowing:
		if err := s.repo.DeleteByTemplateIDFromDate(ctx, templateID, task.ScheduledDate); err != nil {
			return err
		}

		if err := s.truncateTemplateEndDate(ctx, templateID, task.ScheduledDate, s.now()); err != nil {
			return err
		}

	case ScopeAll:
		if err := s.repo.DeleteByTemplateID(ctx, templateID); err != nil {
			return err
		}

		if err := s.templateRepo.Delete(ctx, templateID); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) truncateTemplateEndDate(ctx context.Context, templateID int64, cutoff time.Time, now time.Time) error {
	tmpl, err := s.templateRepo.GetByID(ctx, templateID)
	if err != nil {
		return err
	}

	newEnd := cutoff.AddDate(0, 0, -1)

	if !newEnd.Before(tmpl.StartDate) {
		tmpl.EndDate = &newEnd
		tmpl.UpdatedAt = now
		_, err = s.templateRepo.Update(ctx, tmpl)

		return err
	}

	return s.templateRepo.Delete(ctx, templateID)
}

func (s *Service) List(ctx context.Context, input ListInput) ([]taskdomain.Task, error) {
	if input.From != nil && input.To != nil {
		if err := s.ensureInstancesExist(ctx, *input.From, *input.To); err != nil {
			return nil, err
		}
	}

	return s.repo.List(ctx, input.From, input.To)
}

func validateCreateInput(input CreateInput) (CreateInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return CreateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if input.ScheduledDate.IsZero() {
		return CreateInput{}, fmt.Errorf("%w: scheduled_date is required", ErrInvalidInput)
	}

	input.ScheduledDate = taskdomain.TruncateToDate(input.ScheduledDate)

	if input.Status == "" {
		input.Status = taskdomain.StatusNew
	}

	if !input.Status.Valid() {
		return CreateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	if input.Period != nil {
		period := taskdomain.Period{
			Type:       input.Period.Type,
			Interval:   input.Period.Interval,
			DayOfMonth: input.Period.DayOfMonth,
			Dates:      input.Period.Dates,
			Parity:     input.Period.Parity,
		}

		if err := period.Valid(); err != nil {
			return CreateInput{}, fmt.Errorf("%w: %s", ErrInvalidInput, err)
		}

		if input.Period.EndDate != nil && !input.Period.EndDate.After(input.ScheduledDate) {
			return CreateInput{}, fmt.Errorf("%w: period end_date must be after scheduled_date", ErrInvalidInput)
		}
	}

	return input, nil
}

func validateUpdateInput(input UpdateInput) (UpdateInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return UpdateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if !input.Status.Valid() {
		return UpdateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	if input.Scope == "" {
		input.Scope = ScopeThis
	}

	if !input.Scope.valid() {
		return UpdateInput{}, fmt.Errorf("%w: invalid scope %q", ErrInvalidInput, input.Scope)
	}

	return input, nil
}
