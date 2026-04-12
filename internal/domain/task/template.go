package task

import "time"

type Template struct {
	ID          int64
	Title       string
	Description string
	Period      Period
	StartDate   time.Time
	EndDate     *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
