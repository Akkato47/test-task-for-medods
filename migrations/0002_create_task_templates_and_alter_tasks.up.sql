CREATE TABLE IF NOT EXISTS task_templates (
    id            BIGSERIAL PRIMARY KEY,
    title         TEXT        NOT NULL,
    description   TEXT        NOT NULL DEFAULT '',
    period_type   TEXT        NOT NULL,
    period_config JSONB       NOT NULL DEFAULT '{}',
    start_date    DATE        NOT NULL,
    end_date      DATE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE tasks
    ADD COLUMN IF NOT EXISTS template_id    BIGINT REFERENCES task_templates (id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS scheduled_date DATE NOT NULL DEFAULT CURRENT_DATE;

CREATE INDEX IF NOT EXISTS idx_tasks_template_id    ON tasks (template_id);
CREATE INDEX IF NOT EXISTS idx_tasks_scheduled_date ON tasks (scheduled_date);

-- Prevents duplicate instances for the same template+date.
-- NULL template_id (manual tasks) is exempt: NULLs are never considered equal in UNIQUE constraints.
CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_template_scheduled_unique
    ON tasks (template_id, scheduled_date)
    WHERE template_id IS NOT NULL;
