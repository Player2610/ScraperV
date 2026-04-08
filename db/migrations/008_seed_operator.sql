-- +goose Up
-- +goose StatementBegin
-- Seed a test operator user for development.
-- Credentials: email=operator@protou.co  password=operator123
-- bcrypt hash at cost 12 of "operator123"
INSERT INTO users (email, name, password_hash)
VALUES (
  'operator@protou.co',
  'Operador Protou',
  '$2a$12$0GnDRKJFrpQTKv77PRzShuRaZuVC5lzNs3.Hr30eLKNnc69yXZo32'
)
ON CONFLICT (email) DO NOTHING;
-- +goose StatementEnd

-- +goose StatementBegin
INSERT INTO operators (user_id, role)
SELECT id, 'admin'
FROM users
WHERE email = 'operator@protou.co'
ON CONFLICT (user_id) DO NOTHING;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM operators WHERE user_id = (SELECT id FROM users WHERE email = 'operator@protou.co');
DELETE FROM users WHERE email = 'operator@protou.co';
-- +goose StatementEnd
