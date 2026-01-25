-- +goose Up
ALTER TABLE jobs ADD COLUMN description TEXT;

-- +goose Down
ALTER TABLE jobs DROP COLUMN description;
