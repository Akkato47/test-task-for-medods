package task

import (
	"context"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Scope string

const (
	ScopeThis             Scope = "this"
	ScopeThisAndFollowing Scope = "this_and_following"
	ScopeAll              Scope = "all"
)

func (s Scope) valid() bool {
	switch s {
	case ScopeThis, ScopeThisAndFollowing, ScopeAll:
		return true
	default:
		return false
	}
}

type PeriodInput struct {
	Type       taskdomain.PeriodType
	Interval   int
	DayOfMonth int
	Dates      []time.Time
	Parity     taskdomain.Parity
	EndDate    *time.Time
}

type CreateInput struct {
	Title         string
	Description   string
	Status        taskdomain.Status
	ScheduledDate time.Time
	Period        *PeriodInput
}

type UpdateInput struct {
	Title       string
	Description string
	Status      taskdomain.Status
	Scope       Scope
}

type ListInput struct {
	From *time.Time
	To   *time.Time
}

type Repository interface {
	Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
	CreateBatch(ctx context.Context, tasks []*taskdomain.Task) ([]taskdomain.Task, error)
	CreateBatchIgnoreDuplicates(ctx context.Context, tasks []*taskdomain.Task) error
	GetByID(ctx context.Context, id int64) (*taskdomain.Task, error)
	Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
	UpdateByTemplateID(ctx context.Context, templateID int64, patch taskdomain.TaskPatch, now time.Time) error
	UpdateByTemplateIDFromDate(ctx context.Context, templateID int64, fromDate time.Time, patch taskdomain.TaskPatch, now time.Time) error
	Delete(ctx context.Context, id int64) error
	DeleteByTemplateID(ctx context.Context, templateID int64) error
	DeleteByTemplateIDFromDate(ctx context.Context, templateID int64, fromDate time.Time) error
	List(ctx context.Context, from, to *time.Time) ([]taskdomain.Task, error)
}

type TemplateRepository interface {
	Create(ctx context.Context, template *taskdomain.Template) (*taskdomain.Template, error)
	GetByID(ctx context.Context, id int64) (*taskdomain.Template, error)
	Update(ctx context.Context, template *taskdomain.Template) (*taskdomain.Template, error)
	Delete(ctx context.Context, id int64) error
	ListActive(ctx context.Context) ([]taskdomain.Template, error)
}

type Usecase interface {
	Create(ctx context.Context, input CreateInput) ([]taskdomain.Task, error)
	GetByID(ctx context.Context, id int64) (*taskdomain.Task, error)
	Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error)
	Delete(ctx context.Context, id int64, scope Scope) error
	List(ctx context.Context, input ListInput) ([]taskdomain.Task, error)
}
