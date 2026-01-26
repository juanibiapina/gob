-- +goose Up
ALTER TABLE jobs ADD COLUMN blocked INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE jobs DROP COLUMN blocked;
