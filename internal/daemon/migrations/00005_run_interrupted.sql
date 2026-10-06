-- +goose Up
ALTER TABLE runs ADD COLUMN interrupted INTEGER;

-- +goose Down
ALTER TABLE runs DROP COLUMN interrupted;
