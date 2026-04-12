package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type TemplateRepository struct {
	pool *pgxpool.Pool
}

func NewTemplateRepository(pool *pgxpool.Pool) *TemplateRepository {
	return &TemplateRepository{pool: pool}
}

func (r *TemplateRepository) Create(ctx context.Context, t *taskdomain.Template) (*taskdomain.Template, error) {
	config, err := marshalPeriodConfig(t.Period)
	if err != nil {
		return nil, err
	}

	const query = `
		INSERT INTO task_templates (title, description, period_type, period_config, start_date, end_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, title, description, period_type, period_config, start_date, end_date, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		t.Title, t.Description,
		string(t.Period.Type), config,
		t.StartDate, t.EndDate,
		t.CreatedAt, t.UpdatedAt,
	)

	return scanTemplate(row)
}

func (r *TemplateRepository) GetByID(ctx context.Context, id int64) (*taskdomain.Template, error) {
	const query = `
		SELECT id, title, description, period_type, period_config, start_date, end_date, created_at, updated_at
		FROM task_templates
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	tmpl, err := scanTemplate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrTemplateNotFound
		}

		return nil, err
	}

	return tmpl, nil
}

func (r *TemplateRepository) Update(ctx context.Context, t *taskdomain.Template) (*taskdomain.Template, error) {
	config, err := marshalPeriodConfig(t.Period)
	if err != nil {
		return nil, err
	}

	const query = `
		UPDATE task_templates
		SET title         = $1,
		    description   = $2,
		    period_type   = $3,
		    period_config = $4,
		    start_date    = $5,
		    end_date      = $6,
		    updated_at    = $7
		WHERE id = $8
		RETURNING id, title, description, period_type, period_config, start_date, end_date, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query,
		t.Title, t.Description,
		string(t.Period.Type), config,
		t.StartDate, t.EndDate,
		t.UpdatedAt, t.ID,
	)

	tmpl, err := scanTemplate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrTemplateNotFound
		}

		return nil, err
	}

	return tmpl, nil
}

func (r *TemplateRepository) ListActive(ctx context.Context) ([]taskdomain.Template, error) {
	const query = `
		SELECT id, title, description, period_type, period_config, start_date, end_date, created_at, updated_at
		FROM task_templates
		WHERE end_date IS NULL OR end_date >= CURRENT_DATE
		ORDER BY id
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []taskdomain.Template
	for rows.Next() {
		tmpl, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}

		templates = append(templates, *tmpl)
	}

	return templates, rows.Err()
}

func (r *TemplateRepository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM task_templates WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return taskdomain.ErrTemplateNotFound
	}

	return nil
}

type periodConfigJSON struct {
	Interval   int      `json:"interval,omitempty"`
	DayOfMonth int      `json:"day_of_month,omitempty"`
	Dates      []string `json:"dates,omitempty"`
	Parity     string   `json:"parity,omitempty"`
}

func marshalPeriodConfig(p taskdomain.Period) ([]byte, error) {
	cfg := periodConfigJSON{
		Interval:   p.Interval,
		DayOfMonth: p.DayOfMonth,
		Parity:     string(p.Parity),
	}

	for _, d := range p.Dates {
		cfg.Dates = append(cfg.Dates, d.UTC().Format("2006-01-02"))
	}

	return json.Marshal(cfg)
}

func unmarshalPeriodConfig(periodType taskdomain.PeriodType, raw []byte) (taskdomain.Period, error) {
	var cfg periodConfigJSON
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return taskdomain.Period{}, err
	}

	p := taskdomain.Period{
		Type:       periodType,
		Interval:   cfg.Interval,
		DayOfMonth: cfg.DayOfMonth,
		Parity:     taskdomain.Parity(cfg.Parity),
	}

	for _, s := range cfg.Dates {
		d, err := time.Parse("2006-01-02", s)
		if err != nil {
			return taskdomain.Period{}, err
		}

		p.Dates = append(p.Dates, d.UTC())
	}

	return p, nil
}

type templateScanner interface {
	Scan(dest ...any) error
}

func scanTemplate(scanner templateScanner) (*taskdomain.Template, error) {
	var (
		tmpl       taskdomain.Template
		periodType string
		configRaw  []byte
		startDate  time.Time
		endDate    *time.Time
	)

	if err := scanner.Scan(
		&tmpl.ID,
		&tmpl.Title,
		&tmpl.Description,
		&periodType,
		&configRaw,
		&startDate,
		&endDate,
		&tmpl.CreatedAt,
		&tmpl.UpdatedAt,
	); err != nil {
		return nil, err
	}

	tmpl.StartDate = startDate.UTC()
	tmpl.EndDate = endDate

	period, err := unmarshalPeriodConfig(taskdomain.PeriodType(periodType), configRaw)
	if err != nil {
		return nil, err
	}

	tmpl.Period = period

	return &tmpl, nil
}
