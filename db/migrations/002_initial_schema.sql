-- +goose Up

-- +goose StatementBegin
-- ═══════════════════════════════════════
-- CATALOG
-- ═══════════════════════════════════════

CREATE TABLE stores (
  id         BIGSERIAL PRIMARY KEY,
  name       TEXT NOT NULL,
  base_url   TEXT NOT NULL,
  lat        NUMERIC(9,6),
  lng        NUMERIC(9,6),
  is_active  BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE scrape_rules (
  id                  BIGSERIAL PRIMARY KEY,
  store_id            BIGINT NOT NULL REFERENCES stores(id),
  catalog_url_pattern TEXT NOT NULL,
  item_selector       TEXT NOT NULL,
  price_selector      TEXT NOT NULL,
  name_selector       TEXT NOT NULL,
  image_selector      TEXT,
  stock_selector      TEXT,
  sku_selector        TEXT,
  pagination_selector TEXT,
  headers_json        JSONB,
  delay_ms            INT NOT NULL DEFAULT 2000,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (store_id)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE categories (
  id         BIGSERIAL PRIMARY KEY,
  name       TEXT NOT NULL,
  slug       TEXT NOT NULL UNIQUE,
  parent_id  BIGINT REFERENCES categories(id),
  icon_url   TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_categories_parent ON categories(parent_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE tags (
  id   BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  slug TEXT NOT NULL UNIQUE
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TYPE stock_signal_enum AS ENUM (
  'in_stock', 'out_of_stock', 'unknown', 'price_on_request'
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE listings (
  id              BIGSERIAL PRIMARY KEY,
  store_id        BIGINT NOT NULL REFERENCES stores(id),
  external_sku    TEXT NOT NULL,
  name            TEXT NOT NULL,
  description     TEXT,
  price_cop       INT,
  image_url       TEXT,
  product_url     TEXT NOT NULL,
  stock_signal    stock_signal_enum NOT NULL DEFAULT 'unknown',
  category_id     BIGINT REFERENCES categories(id),
  last_scraped_at TIMESTAMPTZ,
  is_active       BOOLEAN NOT NULL DEFAULT true,
  search_vector   TSVECTOR,
  product_id      BIGINT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (store_id, external_sku)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_listings_search   ON listings USING GIN(search_vector);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_listings_category ON listings(category_id) WHERE is_active = true;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_listings_store    ON listings(store_id) WHERE is_active = true;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_listings_stock    ON listings(stock_signal) WHERE is_active = true;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION listings_search_vector_update() RETURNS trigger AS $$
BEGIN
  NEW.search_vector :=
    setweight(to_tsvector('spanish', coalesce(NEW.name, '')), 'A') ||
    setweight(to_tsvector('spanish', coalesce(NEW.description, '')), 'B');
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER listings_search_vector_trigger
  BEFORE INSERT OR UPDATE OF name, description ON listings
  FOR EACH ROW EXECUTE FUNCTION listings_search_vector_update();
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE listing_tags (
  listing_id BIGINT NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
  tag_id     BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (listing_id, tag_id)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE price_history (
  id           BIGSERIAL PRIMARY KEY,
  listing_id   BIGINT NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
  price_cop    INT,
  stock_signal stock_signal_enum NOT NULL,
  scraped_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_price_history_listing ON price_history(listing_id, scraped_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE products (
  id             BIGSERIAL PRIMARY KEY,
  canonical_name TEXT NOT NULL,
  description    TEXT,
  category_id    BIGINT REFERENCES categories(id),
  embedding      VECTOR(1536),
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE compatibility_notes (
  id           BIGSERIAL PRIMARY KEY,
  product_id_a BIGINT NOT NULL REFERENCES products(id),
  product_id_b BIGINT NOT NULL REFERENCES products(id),
  note         TEXT NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose StatementBegin
-- ═══════════════════════════════════════
-- SCRAPING
-- ═══════════════════════════════════════

CREATE TYPE scrape_job_status AS ENUM ('running', 'success', 'partial', 'failed');
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE scrape_jobs (
  id               BIGSERIAL PRIMARY KEY,
  store_id         BIGINT NOT NULL REFERENCES stores(id),
  started_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  finished_at      TIMESTAMPTZ,
  status           scrape_job_status NOT NULL DEFAULT 'running',
  listings_found   INT NOT NULL DEFAULT 0,
  listings_updated INT NOT NULL DEFAULT 0,
  listings_new     INT NOT NULL DEFAULT 0,
  error_message    TEXT
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_scrape_jobs_store ON scrape_jobs(store_id, started_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
-- ═══════════════════════════════════════
-- USERS
-- ═══════════════════════════════════════

CREATE TABLE users (
  id            BIGSERIAL PRIMARY KEY,
  email         TEXT NOT NULL UNIQUE,
  phone         TEXT,
  name          TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  is_active     BOOLEAN NOT NULL DEFAULT true,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_users_email ON users(email);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE addresses (
  id           BIGSERIAL PRIMARY KEY,
  user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  label        TEXT,
  full_address TEXT NOT NULL,
  reference    TEXT,
  lat          NUMERIC(9,6),
  lng          NUMERIC(9,6),
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_addresses_user ON addresses(user_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TYPE operator_role_enum AS ENUM ('operator', 'admin');
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE operators (
  id         BIGSERIAL PRIMARY KEY,
  user_id    BIGINT NOT NULL REFERENCES users(id) UNIQUE,
  role       operator_role_enum NOT NULL DEFAULT 'operator',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE operator_sessions (
  token       CHAR(64) PRIMARY KEY,
  operator_id BIGINT NOT NULL REFERENCES operators(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at  TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '8 hours'
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_operator_sessions_expiry ON operator_sessions(expires_at);
-- +goose StatementEnd

-- +goose StatementBegin
-- ═══════════════════════════════════════
-- CART
-- ═══════════════════════════════════════

CREATE TABLE carts (
  id         BIGSERIAL PRIMARY KEY,
  user_id    BIGINT NOT NULL REFERENCES users(id) UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE cart_items (
  id         BIGSERIAL PRIMARY KEY,
  cart_id    BIGINT NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
  listing_id BIGINT NOT NULL REFERENCES listings(id),
  quantity   INT NOT NULL DEFAULT 1 CHECK (quantity > 0),
  added_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (cart_id, listing_id)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_cart_items_cart ON cart_items(cart_id);
-- +goose StatementEnd

-- +goose StatementBegin
-- ═══════════════════════════════════════
-- ORDERS
-- ═══════════════════════════════════════

CREATE TYPE order_status_enum AS ENUM (
  'pending_confirmation', 'confirmed', 'purchasing',
  'in_delivery', 'delivered', 'cancelled', 'failed'
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TYPE payment_method_enum AS ENUM (
  'nequi', 'daviplata', 'efectivo', 'llaves_breve'
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE orders (
  id                        BIGSERIAL PRIMARY KEY,
  user_id                   BIGINT NOT NULL REFERENCES users(id),
  status                    order_status_enum NOT NULL DEFAULT 'pending_confirmation',
  delivery_address_snapshot JSONB NOT NULL,
  subtotal_cop              INT NOT NULL,
  delivery_fee_cop          INT NOT NULL,
  total_cop                 INT NOT NULL,
  payment_method            payment_method_enum NOT NULL,
  notes                     TEXT,
  created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_orders_user   ON orders(user_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_orders_status ON orders(status, created_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE order_items (
  id                     BIGSERIAL PRIMARY KEY,
  order_id               BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  listing_id             BIGINT REFERENCES listings(id) ON DELETE SET NULL,
  listing_name_snapshot  TEXT NOT NULL,
  listing_store_snapshot TEXT NOT NULL,
  price_snapshot_cop     INT NOT NULL,
  quantity               INT NOT NULL DEFAULT 1 CHECK (quantity > 0),
  is_cancelled           BOOLEAN NOT NULL DEFAULT false,
  created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_order_items_order ON order_items(order_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE order_events (
  id          BIGSERIAL PRIMARY KEY,
  order_id    BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  from_status order_status_enum,
  to_status   order_status_enum NOT NULL,
  actor_id    BIGINT,
  note        TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_order_events_order ON order_events(order_id, created_at DESC);
-- +goose StatementEnd

-- +goose StatementBegin
-- ═══════════════════════════════════════
-- DELIVERY
-- ═══════════════════════════════════════

CREATE TABLE delivery_fee_brackets (
  id              BIGSERIAL PRIMARY KEY,
  distance_km_min NUMERIC(5,2) NOT NULL,
  distance_km_max NUMERIC(5,2),
  fee_cop         INT NOT NULL,
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE delivery_config (
  id                       INT PRIMARY KEY DEFAULT 1,
  multi_store_discount_pct INT NOT NULL DEFAULT 30,
  updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (id = 1)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE couriers (
  id         BIGSERIAL PRIMARY KEY,
  name       TEXT NOT NULL,
  phone      TEXT NOT NULL,
  is_active  BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE deliveries (
  id               BIGSERIAL PRIMARY KEY,
  order_id         BIGINT NOT NULL REFERENCES orders(id) UNIQUE,
  courier_id       BIGINT REFERENCES couriers(id),
  assigned_at      TIMESTAMPTZ,
  picked_up_at     TIMESTAMPTZ,
  delivered_at     TIMESTAMPTZ,
  delivery_fee_cop INT NOT NULL,
  route_stores     JSONB,
  notes            TEXT
);
-- +goose StatementEnd

-- +goose StatementBegin
-- ═══════════════════════════════════════
-- PAYMENTS
-- ═══════════════════════════════════════

CREATE TABLE payment_records (
  id                      BIGSERIAL PRIMARY KEY,
  order_id                BIGINT NOT NULL REFERENCES orders(id) UNIQUE,
  method                  payment_method_enum NOT NULL,
  amount_cop              INT NOT NULL,
  received_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  received_by_operator_id BIGINT REFERENCES users(id),
  notes                   TEXT
);
-- +goose StatementEnd

-- +goose StatementBegin
-- ═══════════════════════════════════════
-- NOTIFICATIONS
-- ═══════════════════════════════════════

CREATE TYPE notification_channel_enum AS ENUM ('email');
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TYPE notification_status_enum AS ENUM ('sent', 'failed');
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE notification_logs (
  id            BIGSERIAL PRIMARY KEY,
  order_id      BIGINT NOT NULL REFERENCES orders(id),
  user_id       BIGINT NOT NULL REFERENCES users(id),
  channel       notification_channel_enum NOT NULL DEFAULT 'email',
  event         TEXT NOT NULL,
  sent_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  status        notification_status_enum NOT NULL,
  error_message TEXT
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_notification_logs_order ON notification_logs(order_id);
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
DROP INDEX IF EXISTS idx_notification_logs_order;
DROP TABLE IF EXISTS notification_logs;
DROP TYPE IF EXISTS notification_status_enum;
DROP TYPE IF EXISTS notification_channel_enum;
DROP TABLE IF EXISTS payment_records;
DROP TABLE IF EXISTS deliveries;
DROP TABLE IF EXISTS couriers;
DROP TABLE IF EXISTS delivery_config;
DROP TABLE IF EXISTS delivery_fee_brackets;
DROP INDEX IF EXISTS idx_order_events_order;
DROP TABLE IF EXISTS order_events;
DROP INDEX IF EXISTS idx_order_items_order;
DROP TABLE IF EXISTS order_items;
DROP INDEX IF EXISTS idx_orders_status;
DROP INDEX IF EXISTS idx_orders_user;
DROP TABLE IF EXISTS orders;
DROP TYPE IF EXISTS payment_method_enum;
DROP TYPE IF EXISTS order_status_enum;
DROP INDEX IF EXISTS idx_cart_items_cart;
DROP TABLE IF EXISTS cart_items;
DROP TABLE IF EXISTS carts;
DROP INDEX IF EXISTS idx_operator_sessions_expiry;
DROP TABLE IF EXISTS operator_sessions;
DROP TABLE IF EXISTS operators;
DROP TYPE IF EXISTS operator_role_enum;
DROP INDEX IF EXISTS idx_addresses_user;
DROP TABLE IF EXISTS addresses;
DROP INDEX IF EXISTS idx_users_email;
DROP TABLE IF EXISTS users;
DROP INDEX IF EXISTS idx_scrape_jobs_store;
DROP TABLE IF EXISTS scrape_jobs;
DROP TYPE IF EXISTS scrape_job_status;
DROP TABLE IF EXISTS compatibility_notes;
DROP TABLE IF EXISTS products;
DROP INDEX IF EXISTS idx_price_history_listing;
DROP TABLE IF EXISTS price_history;
DROP TABLE IF EXISTS listing_tags;
DROP TRIGGER IF EXISTS listings_search_vector_trigger ON listings;
DROP FUNCTION IF EXISTS listings_search_vector_update();
DROP INDEX IF EXISTS idx_listings_stock;
DROP INDEX IF EXISTS idx_listings_store;
DROP INDEX IF EXISTS idx_listings_category;
DROP INDEX IF EXISTS idx_listings_search;
DROP TABLE IF EXISTS listings;
DROP TYPE IF EXISTS stock_signal_enum;
DROP TABLE IF EXISTS tags;
DROP INDEX IF EXISTS idx_categories_parent;
DROP TABLE IF EXISTS categories;
DROP TABLE IF EXISTS scrape_rules;
DROP TABLE IF EXISTS stores;
-- +goose StatementEnd
