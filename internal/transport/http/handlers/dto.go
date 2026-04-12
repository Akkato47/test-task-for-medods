package handlers

import (
	"fmt"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
	taskusecase "example.com/taskservice/internal/usecase/task"
)

const dateLayout = "2006-01-02"

// taskMutationDTO is the request body for Create and Update.
type taskMutationDTO struct {
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Status        taskdomain.Status `json:"status"`
	ScheduledDate string            `json:"scheduled_date"` // "YYYY-MM-DD", required
	Period        *periodDTO        `json:"period,omitempty"`
}

// periodDTO carries the recurrence rule from the client.
// StartDate is taken from the parent taskMutationDTO.ScheduledDate.
type periodDTO struct {
	Type       taskdomain.PeriodType `json:"type"`
	Interval   int                   `json:"interval,omitempty"`
	DayOfMonth int                   `json:"day_of_month,omitempty"`
	Dates      []string              `json:"dates,omitempty"` // "YYYY-MM-DD"
	Parity     taskdomain.Parity     `json:"parity,omitempty"`
	EndDate    *string               `json:"end_date,omitempty"`
}

// taskDTO is the response representation of a single task.
type taskDTO struct {
	ID            int64             `json:"id"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Status        taskdomain.Status `json:"status"`
	TemplateID    *int64            `json:"template_id,omitempty"`
	ScheduledDate string            `json:"scheduled_date"` // "YYYY-MM-DD"
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

func newTaskDTO(task *taskdomain.Task) taskDTO {
	return taskDTO{
		ID:            task.ID,
		Title:         task.Title,
		Description:   task.Description,
		Status:        task.Status,
		TemplateID:    task.TemplateID,
		ScheduledDate: task.ScheduledDate.Format(dateLayout),
		CreatedAt:     task.CreatedAt,
		UpdatedAt:     task.UpdatedAt,
	}
}

// toPeriodInput converts the DTO into a use-case input, parsing date strings.
func (p *periodDTO) toPeriodInput() (*taskusecase.PeriodInput, error) {
	if p == nil {
		return nil, nil
	}

	input := &taskusecase.PeriodInput{
		Type:       p.Type,
		Interval:   p.Interval,
		DayOfMonth: p.DayOfMonth,
		Parity:     p.Parity,
	}

	if p.EndDate != nil {
		endDate, err := time.Parse(dateLayout, *p.EndDate)
		if err != nil {
			return nil, fmt.Errorf("period end_date: %w", err)
		}

		t := endDate.UTC()
		input.EndDate = &t
	}

	for _, s := range p.Dates {
		d, err := time.Parse(dateLayout, s)
		if err != nil {
			return nil, fmt.Errorf("period dates: %w", err)
		}

		input.Dates = append(input.Dates, d.UTC())
	}

	return input, nil
}
