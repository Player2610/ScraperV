-- +goose Up
-- +goose StatementBegin
INSERT INTO delivery_config DEFAULT VALUES
ON CONFLICT (id) DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO delivery_fee_brackets (distance_km_min, distance_km_max, fee_cop) VALUES
  (0,    3,    5000),
  (3,    8,    8000),
  (8,    15,   12000),
  (15,   25,   16000),
  (25,   NULL, 20000);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM delivery_fee_brackets;
-- +goose StatementEnd

-- +goose StatementBegin
DELETE FROM delivery_config;
-- +goose StatementEnd
