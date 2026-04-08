-- +goose Up
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS habeas_data_consent_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE users
  DROP COLUMN IF EXISTS habeas_data_consent_at,
  DROP COLUMN IF EXISTS deleted_at;
