-- +goose Up

-- +goose StatementBegin
-- Root categories (parent_id = NULL)
INSERT INTO categories (name, slug, parent_id) VALUES
  ('Resistencias',                  'resistencias',                 NULL),
  ('Capacitores',                   'capacitores',                  NULL),
  ('Semiconductores',               'semiconductores',              NULL),
  ('Circuitos Integrados',          'circuitos-integrados',         NULL),
  ('Microcontroladores y Módulos',  'microcontroladores-y-modulos', NULL),
  ('Sensores',                      'sensores',                     NULL),
  ('Comunicaciones',                'comunicaciones',               NULL),
  ('Motores y Actuadores',          'motores-y-actuadores',         NULL),
  ('Conectores y Cables',           'conectores-y-cables',          NULL),
  ('Prototipos',                    'prototipos',                   NULL),
  ('Fuentes de Alimentación',       'fuentes-de-alimentacion',      NULL),
  ('Displays y Audio',              'displays-y-audio',             NULL);
-- +goose StatementEnd

-- +goose StatementBegin
-- Subcategories — Resistencias
INSERT INTO categories (name, slug, parent_id)
SELECT sub.name, sub.slug, c.id
FROM categories c
JOIN (VALUES
  ('Resistencias 1/4W',  'resistencias-1-4w'),
  ('Resistencias 1/2W',  'resistencias-1-2w'),
  ('Resistencias SMD',   'resistencias-smd')
) AS sub(name, slug) ON c.slug = 'resistencias';
-- +goose StatementEnd

-- +goose StatementBegin
-- Subcategories — Capacitores
INSERT INTO categories (name, slug, parent_id)
SELECT sub.name, sub.slug, c.id
FROM categories c
JOIN (VALUES
  ('Electrolíticos', 'capacitores-electroliticos'),
  ('Cerámicos',      'capacitores-ceramicos'),
  ('Tántalo',        'capacitores-tantalo')
) AS sub(name, slug) ON c.slug = 'capacitores';
-- +goose StatementEnd

-- +goose StatementBegin
-- Subcategories — Semiconductores
INSERT INTO categories (name, slug, parent_id)
SELECT sub.name, sub.slug, c.id
FROM categories c
JOIN (VALUES
  ('Diodos',      'diodos'),
  ('Transistores','transistores'),
  ('MOSFETs',     'mosfets'),
  ('Tiristores',  'tiristores')
) AS sub(name, slug) ON c.slug = 'semiconductores';
-- +goose StatementEnd

-- +goose StatementBegin
-- Subcategories — Circuitos Integrados
INSERT INTO categories (name, slug, parent_id)
SELECT sub.name, sub.slug, c.id
FROM categories c
JOIN (VALUES
  ('Amplificadores Operacionales', 'amplificadores-operacionales'),
  ('Reguladores de Voltaje',       'reguladores-de-voltaje'),
  ('Lógica Digital',               'logica-digital'),
  ('Microcontroladores CI',        'microcontroladores-ci')
) AS sub(name, slug) ON c.slug = 'circuitos-integrados';
-- +goose StatementEnd

-- +goose StatementBegin
-- Subcategories — Microcontroladores y Módulos
INSERT INTO categories (name, slug, parent_id)
SELECT sub.name, sub.slug, c.id
FROM categories c
JOIN (VALUES
  ('Arduino',          'arduino'),
  ('ESP32 / ESP8266',  'esp32-esp8266'),
  ('Raspberry Pi',     'raspberry-pi'),
  ('STM32',            'stm32')
) AS sub(name, slug) ON c.slug = 'microcontroladores-y-modulos';
-- +goose StatementEnd

-- +goose StatementBegin
-- Subcategories — Sensores
INSERT INTO categories (name, slug, parent_id)
SELECT sub.name, sub.slug, c.id
FROM categories c
JOIN (VALUES
  ('Temperatura y Humedad',   'sensores-temperatura-humedad'),
  ('Movimiento y Proximidad', 'sensores-movimiento-proximidad'),
  ('Luz y Color',             'sensores-luz-color'),
  ('Presión y Gas',           'sensores-presion-gas'),
  ('Corriente y Voltaje',     'sensores-corriente-voltaje')
) AS sub(name, slug) ON c.slug = 'sensores';
-- +goose StatementEnd

-- +goose StatementBegin
-- Subcategories — Comunicaciones
INSERT INTO categories (name, slug, parent_id)
SELECT sub.name, sub.slug, c.id
FROM categories c
JOIN (VALUES
  ('WiFi',        'comunicaciones-wifi'),
  ('Bluetooth',   'comunicaciones-bluetooth'),
  ('RF / LoRa',   'comunicaciones-rf-lora'),
  ('GPS',         'comunicaciones-gps')
) AS sub(name, slug) ON c.slug = 'comunicaciones';
-- +goose StatementEnd

-- +goose StatementBegin
-- Subcategories — Motores y Actuadores
INSERT INTO categories (name, slug, parent_id)
SELECT sub.name, sub.slug, c.id
FROM categories c
JOIN (VALUES
  ('Servos',               'servos'),
  ('Motores DC',           'motores-dc'),
  ('Motores Paso a Paso',  'motores-paso-a-paso'),
  ('Relés',                'reles')
) AS sub(name, slug) ON c.slug = 'motores-y-actuadores';
-- +goose StatementEnd

-- +goose StatementBegin
-- Subcategories — Conectores y Cables
INSERT INTO categories (name, slug, parent_id)
SELECT sub.name, sub.slug, c.id
FROM categories c
JOIN (VALUES
  ('Dupont / Jumper', 'dupont-jumper'),
  ('JST',             'jst'),
  ('Borneras',        'borneras')
) AS sub(name, slug) ON c.slug = 'conectores-y-cables';
-- +goose StatementEnd

-- +goose StatementBegin
-- Subcategories — Prototipos
INSERT INTO categories (name, slug, parent_id)
SELECT sub.name, sub.slug, c.id
FROM categories c
JOIN (VALUES
  ('Protoboards', 'protoboards'),
  ('PCBs',        'pcbs'),
  ('Herramientas','herramientas')
) AS sub(name, slug) ON c.slug = 'prototipos';
-- +goose StatementEnd

-- +goose StatementBegin
-- Subcategories — Fuentes de Alimentación
INSERT INTO categories (name, slug, parent_id)
SELECT sub.name, sub.slug, c.id
FROM categories c
JOIN (VALUES
  ('Baterías',         'baterias'),
  ('Reguladores',      'reguladores'),
  ('Módulos de Carga', 'modulos-de-carga')
) AS sub(name, slug) ON c.slug = 'fuentes-de-alimentacion';
-- +goose StatementEnd

-- +goose StatementBegin
-- Subcategories — Displays y Audio
INSERT INTO categories (name, slug, parent_id)
SELECT sub.name, sub.slug, c.id
FROM categories c
JOIN (VALUES
  ('LCD / OLED',       'lcd-oled'),
  ('LED y Matrices',   'led-matrices'),
  ('Buzzer y Audio',   'buzzer-audio')
) AS sub(name, slug) ON c.slug = 'displays-y-audio';
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
DELETE FROM categories;
-- +goose StatementEnd
