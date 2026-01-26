-- +goose Up
ALTER TABLE jobs ADD COLUMN failure_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE jobs ADD COLUMN success_total_duration_ms INTEGER NOT NULL DEFAULT 0;
ALTER TABLE jobs ADD COLUMN failure_total_duration_ms INTEGER NOT NULL DEFAULT 0;

-- Recalculate failure_count from completed runs with non-zero exit code
UPDATE jobs SET failure_count = (
    SELECT COUNT(*)
    FROM runs
    WHERE runs.job_id = jobs.id
      AND runs.stopped_at IS NOT NULL
      AND runs.exit_code IS NOT NULL
      AND runs.exit_code != 0
);

-- Recalculate success totals
UPDATE jobs SET success_total_duration_ms = COALESCE((
    SELECT SUM((strftime('%s', stopped_at) - strftime('%s', started_at)) * 1000)
    FROM runs
    WHERE runs.job_id = jobs.id
      AND runs.stopped_at IS NOT NULL
      AND runs.exit_code = 0
), 0);

-- Recalculate failure totals (excludes killed)
UPDATE jobs SET failure_total_duration_ms = COALESCE((
    SELECT SUM((strftime('%s', stopped_at) - strftime('%s', started_at)) * 1000)
    FROM runs
    WHERE runs.job_id = jobs.id
      AND runs.stopped_at IS NOT NULL
      AND runs.exit_code IS NOT NULL
      AND runs.exit_code != 0
), 0);

ALTER TABLE jobs DROP COLUMN total_duration_ms;

-- +goose Down
ALTER TABLE jobs ADD COLUMN total_duration_ms INTEGER NOT NULL DEFAULT 0;
ALTER TABLE jobs DROP COLUMN failure_count;
ALTER TABLE jobs DROP COLUMN success_total_duration_ms;
ALTER TABLE jobs DROP COLUMN failure_total_duration_ms;
