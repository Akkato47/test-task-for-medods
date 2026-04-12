package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		INSERT INTO tasks (title, description, status, template_id, scheduled_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, title, description, status, template_id, scheduled_date, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		task.Title, task.Description, task.Status,
		task.TemplateID, task.ScheduledDate,
		task.CreatedAt, task.UpdatedAt,
	)

	created, err := scanTask(row)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (r *Repository) CreateBatch(ctx context.Context, tasks []*taskdomain.Task) ([]taskdomain.Task, error) {
	if len(tasks) == 0 {
		return nil, nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	const query = `
		INSERT INTO tasks (title, description, status, template_id, scheduled_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, title, description, status, template_id, scheduled_date, created_at, updated_at
	`

	result := make([]taskdomain.Task, 0, len(tasks))
	for _, t := range tasks {
		row := tx.QueryRow(ctx, query,
			t.Title, t.Description, t.Status,
			t.TemplateID, t.ScheduledDate,
			t.CreatedAt, t.UpdatedAt,
		)

		created, err := scanTask(row)
		if err != nil {
			return nil, err
		}

		result = append(result, *created)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return result, nil
}

func (r *Repository) CreateBatchIgnoreDuplicates(ctx context.Context, tasks []*taskdomain.Task) error {
	if len(tasks) == 0 {
		return nil
	}

	const query = `
		INSERT INTO tasks (title, description, status, template_id, scheduled_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (template_id, scheduled_date) WHERE template_id IS NOT NULL DO NOTHING
	`

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, t := range tasks {
		if _, err := tx.Exec(ctx, query,
			t.Title, t.Description, t.Status,
			t.TemplateID, t.ScheduledDate,
			t.CreatedAt, t.UpdatedAt,
		); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, template_id, scheduled_date, created_at, updated_at
		FROM tasks
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	found, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return found, nil
}

func (r *Repository) Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		UPDATE tasks
		SET title         = $1,
		    description   = $2,
		    status        = $3,
		    updated_at    = $4
		WHERE id = $5
		RETURNING id, title, description, status, template_id, scheduled_date, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		task.Title, task.Description, task.Status,
		task.UpdatedAt, task.ID,
	)

	updated, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return updated, nil
}

func (r *Repository) UpdateByTemplateID(ctx context.Context, templateID int64, patch taskdomain.TaskPatch, now time.Time) error {
	const query = `
		UPDATE tasks
		SET title       = $1,
		    description = $2,
		    status      = $3,
		    updated_at  = $4
		WHERE template_id = $5
	`

	_, err := r.pool.Exec(ctx, query, patch.Title, patch.Description, string(patch.Status), now, templateID)

	return err
}

func (r *Repository) UpdateByTemplateIDFromDate(ctx context.Context, templateID int64, fromDate time.Time, patch taskdomain.TaskPatch, now time.Time) error {
	const query = `
		UPDATE tasks
		SET title       = $1,
		    description = $2,
		    status      = $3,
		    updated_at  = $4
		WHERE template_id = $5
		  AND scheduled_date >= $6
	`

	_, err := r.pool.Exec(ctx, query, patch.Title, patch.Description, string(patch.Status), now, templateID, fromDate)

	return err
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM tasks WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}

	return nil
}

func (r *Repository) DeleteByTemplateID(ctx context.Context, templateID int64) error {
	const query = `DELETE FROM tasks WHERE template_id = $1`

	_, err := r.pool.Exec(ctx, query, templateID)

	return err
}

func (r *Repository) DeleteByTemplateIDFromDate(ctx context.Context, templateID int64, fromDate time.Time) error {
	const query = `DELETE FROM tasks WHERE template_id = $1 AND scheduled_date >= $2`

	_, err := r.pool.Exec(ctx, query, templateID, fromDate)

	return err
}

func (r *Repository) List(ctx context.Context, from, to *time.Time) ([]taskdomain.Task, error) {
	query := `
		SELECT id, title, description, status, template_id, scheduled_date, created_at, updated_at
		FROM tasks
	`

	args := make([]any, 0, 2)

	switch {
	case from != nil && to != nil:
		query += ` WHERE scheduled_date >= $1 AND scheduled_date <= $2`
		args = append(args, from, to)
	case from != nil:
		query += ` WHERE scheduled_date >= $1`
		args = append(args, from)
	case to != nil:
		query += ` WHERE scheduled_date <= $1`
		args = append(args, to)
	}

	query += ` ORDER BY scheduled_date ASC NULLS LAST, id DESC`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]taskdomain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, *task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}

type taskScanner interface {
	Scan(dest ...any) error
}

func scanTask(scanner taskScanner) (*taskdomain.Task, error) {
	var (
		task   taskdomain.Task
		status string
	)

	if err := scanner.Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&status,
		&task.TemplateID,
		&task.ScheduledDate,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		return nil, err
	}

	task.Status = taskdomain.Status(status)

	return &task, nil
}
