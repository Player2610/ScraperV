-- +goose Up

-- +goose StatementBegin
-- Vistronica: domain www.vistronica.com.co does not resolve in DNS (store inactive or migrated).
UPDATE stores SET is_active = false WHERE name = 'Vistronica';
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
UPDATE stores SET is_active = true WHERE name = 'Vistronica';
-- +goose StatementEnd
