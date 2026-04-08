# Modelo de datos

Schema completo con SQL. Las migraciones viven en `db/migrations/`.

---

## Extensions

```sql
CREATE EXTENSION IF NOT EXISTS pgvector;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
```

---

## Catalog

```sql
CREATE TABLE stores (
  id         BIGSERIAL PRIMARY KEY,
  name       TEXT NOT NULL,
  base_url   TEXT NOT NULL,
  lat        NUMERIC(9,6),         -- coordenadas para cálculo de tarifa
  lng        NUMERIC(9,6),
  is_active  BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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

CREATE TABLE categories (
  id         BIGSERIAL PRIMARY KEY,
  name       TEXT NOT NULL,
  slug       TEXT NOT NULL UNIQUE,   -- URL-safe, ej: "resistencias-1-4w"
  parent_id  BIGINT REFERENCES categories(id),
  icon_url   TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE tags (
  id   BIGSERIAL PRIMARY KEY,
  name TEXT NOT NULL,
  slug TEXT NOT NULL UNIQUE
);

CREATE TYPE stock_signal_enum AS ENUM (
  'in_stock', 'out_of_stock', 'unknown', 'price_on_request'
);

CREATE TABLE listings (
  id              BIGSERIAL PRIMARY KEY,
  store_id        BIGINT NOT NULL REFERENCES stores(id),
  external_sku    TEXT NOT NULL,
  name            TEXT NOT NULL,
  description     TEXT,
  price_cop       INT,               -- NULL = price_on_request
  image_url       TEXT,
  product_url     TEXT NOT NULL,
  stock_signal    stock_signal_enum NOT NULL DEFAULT 'unknown',
  category_id     BIGINT REFERENCES categories(id),
  last_scraped_at TIMESTAMPTZ,
  is_active       BOOLEAN NOT NULL DEFAULT true,
  search_vector   TSVECTOR,          -- actualizado por trigger, índice GIN
  product_id      BIGINT,            -- Fase 2: FK a products (nullable)
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (store_id, external_sku)    -- ancla de deduplicación
);
CREATE INDEX idx_listings_search   ON listings USING GIN(search_vector);
CREATE INDEX idx_listings_category ON listings(category_id) WHERE is_active = true;
CREATE INDEX idx_listings_stock    ON listings(stock_signal) WHERE is_active = true;

-- Trigger FTS (idioma: español)
CREATE OR REPLACE FUNCTION listings_search_vector_update() RETURNS trigger AS $$
BEGIN
  NEW.search_vector :=
    setweight(to_tsvector('spanish', coalesce(NEW.name, '')), 'A') ||
    setweight(to_tsvector('spanish', coalesce(NEW.description, '')), 'B');
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER listings_search_vector_trigger
  BEFORE INSERT OR UPDATE OF name, description ON listings
  FOR EACH ROW EXECUTE FUNCTION listings_search_vector_update();

CREATE TABLE listing_tags (
  listing_id BIGINT NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
  tag_id     BIGINT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
  PRIMARY KEY (listing_id, tag_id)
);

CREATE TABLE price_history (
  id           BIGSERIAL PRIMARY KEY,
  listing_id   BIGINT NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
  price_cop    INT,
  stock_signal stock_signal_enum NOT NULL,
  scraped_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_price_history_listing ON price_history(listing_id, scraped_at DESC);

-- Fase 2 (tablas creadas desde el inicio, datos poblados después)
CREATE TABLE products (
  id             BIGSERIAL PRIMARY KEY,
  canonical_name TEXT NOT NULL,
  description    TEXT,
  category_id    BIGINT REFERENCES categories(id),
  embedding      VECTOR(1536),  -- pgvector, NULL hasta Fase 2
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE compatibility_notes (
  id           BIGSERIAL PRIMARY KEY,
  product_id_a BIGINT NOT NULL REFERENCES products(id),
  product_id_b BIGINT NOT NULL REFERENCES products(id),
  note         TEXT NOT NULL,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

---

## Scraping

```sql
CREATE TYPE scrape_job_status AS ENUM ('running', 'success', 'partial', 'failed');

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
CREATE INDEX idx_scrape_jobs_store ON scrape_jobs(store_id, started_at DESC);
```

---

## Users

```sql
CREATE TABLE users (
  id            BIGSERIAL PRIMARY KEY,
  email         TEXT NOT NULL UNIQUE,
  phone         TEXT,
  name          TEXT NOT NULL,
  password_hash TEXT NOT NULL,        -- bcrypt
  is_active     BOOLEAN NOT NULL DEFAULT true,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE addresses (
  id           BIGSERIAL PRIMARY KEY,
  user_id      BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  label        TEXT,
  full_address TEXT NOT NULL,
  reference    TEXT,
  lat          NUMERIC(9,6),           -- geocodificado al guardar
  lng          NUMERIC(9,6),
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_addresses_user ON addresses(user_id);

CREATE TYPE operator_role_enum AS ENUM ('operator', 'admin');

CREATE TABLE operators (
  id         BIGSERIAL PRIMARY KEY,
  user_id    BIGINT NOT NULL REFERENCES users(id) UNIQUE,
  role       operator_role_enum NOT NULL DEFAULT 'operator',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE operator_sessions (
  token       CHAR(64) PRIMARY KEY,   -- 32 bytes random hex
  operator_id BIGINT NOT NULL REFERENCES operators(id),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  expires_at  TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '8 hours'
);
CREATE INDEX idx_operator_sessions_expiry ON operator_sessions(expires_at);
```

---

## Cart

```sql
CREATE TABLE carts (
  id         BIGSERIAL PRIMARY KEY,
  user_id    BIGINT NOT NULL REFERENCES users(id) UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE cart_items (
  id         BIGSERIAL PRIMARY KEY,
  cart_id    BIGINT NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
  listing_id BIGINT NOT NULL REFERENCES listings(id),
  quantity   INT NOT NULL DEFAULT 1 CHECK (quantity > 0),
  added_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (cart_id, listing_id)
);
```

---

## Orders

```sql
CREATE TYPE order_status_enum AS ENUM (
  'pending_confirmation', 'confirmed', 'purchasing',
  'in_delivery', 'delivered', 'cancelled', 'failed'
);
CREATE TYPE payment_method_enum AS ENUM (
  'nequi', 'daviplata', 'efectivo', 'llaves_breve'
);

CREATE TABLE orders (
  id                        BIGSERIAL PRIMARY KEY,
  user_id                   BIGINT NOT NULL REFERENCES users(id),
  status                    order_status_enum NOT NULL DEFAULT 'pending_confirmation',
  delivery_address_snapshot JSONB NOT NULL,  -- inmutable, capturado al crear
  subtotal_cop              INT NOT NULL,
  delivery_fee_cop          INT NOT NULL,
  total_cop                 INT NOT NULL,
  payment_method            payment_method_enum NOT NULL,
  notes                     TEXT,
  created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_orders_user   ON orders(user_id);
CREATE INDEX idx_orders_status ON orders(status, created_at DESC);

CREATE TABLE order_items (
  id                     BIGSERIAL PRIMARY KEY,
  order_id               BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  listing_id             BIGINT REFERENCES listings(id) ON DELETE SET NULL,
  listing_name_snapshot  TEXT NOT NULL,     -- inmutable
  listing_store_snapshot TEXT NOT NULL,     -- inmutable
  price_snapshot_cop     INT NOT NULL,      -- inmutable
  quantity               INT NOT NULL DEFAULT 1 CHECK (quantity > 0),
  is_cancelled           BOOLEAN NOT NULL DEFAULT false,
  created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_order_items_order ON order_items(order_id);

CREATE TABLE order_events (
  id          BIGSERIAL PRIMARY KEY,
  order_id    BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  from_status order_status_enum,
  to_status   order_status_enum NOT NULL,
  actor_id    BIGINT,              -- NULL = sistema; non-null = operator user_id
  note        TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_order_events_order ON order_events(order_id, created_at DESC);
```

---

## Delivery

```sql
CREATE TABLE delivery_fee_brackets (
  id              BIGSERIAL PRIMARY KEY,
  distance_km_min NUMERIC(5,2) NOT NULL,
  distance_km_max NUMERIC(5,2),           -- NULL = sin límite (25+ km)
  fee_cop         INT NOT NULL,
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE delivery_config (
  id                       INT PRIMARY KEY DEFAULT 1,
  multi_store_discount_pct INT NOT NULL DEFAULT 30,
  updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (id = 1)           -- fila singleton
);
INSERT INTO delivery_config DEFAULT VALUES;

CREATE TABLE couriers (
  id         BIGSERIAL PRIMARY KEY,
  name       TEXT NOT NULL,
  phone      TEXT NOT NULL,
  is_active  BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE deliveries (
  id               BIGSERIAL PRIMARY KEY,
  order_id         BIGINT NOT NULL REFERENCES orders(id) UNIQUE,
  courier_id       BIGINT REFERENCES couriers(id),
  assigned_at      TIMESTAMPTZ,
  picked_up_at     TIMESTAMPTZ,
  delivered_at     TIMESTAMPTZ,
  delivery_fee_cop INT NOT NULL,
  route_stores     JSONB,    -- [{store_id, name, address, visit_order}]
  notes            TEXT
);
```

---

## Payments

```sql
CREATE TABLE payment_records (
  id                      BIGSERIAL PRIMARY KEY,
  order_id                BIGINT NOT NULL REFERENCES orders(id) UNIQUE,
  method                  payment_method_enum NOT NULL,
  amount_cop              INT NOT NULL,
  received_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  received_by_operator_id BIGINT REFERENCES users(id),
  notes                   TEXT
);
```

---

## Notifications

```sql
CREATE TYPE notification_channel_enum AS ENUM ('email');
CREATE TYPE notification_status_enum  AS ENUM ('sent', 'failed');

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
CREATE INDEX idx_notification_logs_order ON notification_logs(order_id);
```

---

## Notas de diseño

- **Snapshots inmutables**: `OrderItem.price_snapshot_cop`, `listing_name_snapshot`, `listing_store_snapshot` y `Order.delivery_address_snapshot` (JSONB) nunca se modifican después de la creación.
- **pgvector desde el día 1**: `products.embedding VECTOR(1536)` es `NULL` hasta Fase 2. Activar la extensión luego en producción es costoso.
- **`delivery_config` es un singleton**: la tabla tiene exactamente una fila (`id = 1`). Los brackets y el descuento multi-tienda se editan desde el panel sin redeploy.
- **`operator_sessions` en PostgreSQL**: sin Redis. Las sesiones expiran en 8h y se limpian con un goroutine cada hora en el API.
- **`OrderItem.is_cancelled`**: permite cancelar ítems individuales sin eliminar el registro. Si todos quedan `is_cancelled = true`, la orden pasa a `cancelled`.
- **`listing_id` en `order_items` es nullable** (`ON DELETE SET NULL`): si un listing se elimina de la DB, la orden histórica mantiene los snapshots intactos.
