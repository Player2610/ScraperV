-- +goose Up

-- +goose StatementBegin
INSERT INTO tags (name, slug) VALUES
  ('I2C',                 'i2c'),
  ('SPI',                 'spi'),
  ('UART',                'uart'),
  ('PWM',                 'pwm'),
  ('ADC',                 'adc'),
  ('5V',                  '5v'),
  ('3.3V',                '3-3v'),
  ('12V',                 '12v'),
  ('Analógico',           'analog'),
  ('Digital',             'digital'),
  ('SMD',                 'smd'),
  ('Through-Hole',        'through-hole'),
  ('Principiante',        'beginner-friendly'),
  ('Grove Compatible',    'grove-compatible'),
  ('Arduino Compatible',  'arduino-compatible'),
  ('ESP32 Compatible',    'esp32-compatible');
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
DELETE FROM tags WHERE slug IN (
  'i2c','spi','uart','pwm','adc','5v','3-3v','12v',
  'analog','digital','smd','through-hole','beginner-friendly',
  'grove-compatible','arduino-compatible','esp32-compatible'
);
-- +goose StatementEnd
