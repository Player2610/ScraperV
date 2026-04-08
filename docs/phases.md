# Fases de implementación

Ver estado actual en [progress.md](./progress.md).

---

## Fase 0 — Fundación ✅

**Objetivo:** entorno funcional, nada de negocio todavía.

**Estado:** completa. `go build ./...` y `go test ./...` pasan.

### Lo implementado

| Tarea | Archivo(s) |
|-------|-----------|
| Monorepo Go scaffold | `cmd/api/`, `cmd/scraper/`, `cmd/migrate/`, `internal/*/`, `go.mod` |
| Migraciones (goose) | `db/migrations/001–003_*.sql`, `db/migrate.go` |
| Schema completo en DB | `db/migrations/002_initial_schema.sql` — 20+ tablas, enums, índices, trigger FTS |
| Seed delivery config | `db/migrations/003_seed_delivery_config.sql` — 5 brackets + discount_pct=30 |
| `internal/platform` | `config.go`, `db.go`, `httpserver.go` — chi router + health endpoint |
| Dockerfile multi-target | `Dockerfile` — targets: `api`, `scraper`, `migrate` |
| docker-compose local | `docker-compose.yml` — postgres+pgvector + API |
| CI GitHub Actions | `.github/workflows/ci.yml` — lint + test (unit + integración) + build-check (PRs) |
| CD API | `.github/workflows/deploy-api.yml` — WIF → build+push → migrate → deploy Cloud Run → smoke test |
| CD Scraper | `.github/workflows/deploy-scraper.yml` — WIF → build+push → actualiza Cloud Run Job |
| Setup CI/CD GCP | `docs/gcp-cicd-setup.md` — Artifact Registry + WIF pool/provider + GitHub secrets |
| Makefile | `Makefile` — `help`, `dev`, `build`, `test`, `lint`, `migrate-up/down`, `scraper-dry-run` |
| Astro scaffold | `web/` — SSR + Cloudflare adapter + Tailwind + `api.ts` |

### Pendiente (setup externo)
- **0.1** GCP: crear proyecto, habilitar APIs, configurar IAM
- **0.2** Neon: crear proyecto, obtener `DATABASE_URL`
- **0.8** Cloudflare Pages: conectar repo, configurar build

---

## Fase 1 — Scraper ✅ (código) / ⏳ (deploy)

**Objetivo:** catálogo vivo con datos reales de todas las tiendas disponibles en Bogotá.

**Estado:** código completo, 29 tests pasando. Pendiente: deploy a Cloud Run (requiere GCP).

### Lo implementado

| Tarea | Archivo(s) |
|-------|-----------|
| Discovery de tiendas | `docs/stores-discovery.md` — 9 tiendas documentadas, selectores por plataforma |
| Seed stores + reglas | `db/migrations/004_seed_stores.sql` — Sigmaelectrónica, Electronilab, Vistronica |
| Seed categorías | `db/migrations/005_seed_categories.sql` — ~52 categorías en árbol jerárquico |
| Seed tags | `db/migrations/006_seed_tags.sql` — 16 tags (i2c, spi, uart, 5v, etc.) |
| Tipos del dominio | `internal/scraping/types.go` |
| Repositorio | `internal/scraping/repository.go` |
| Parser HTML | `internal/scraping/parser.go` — goquery, `ParsePrice`, `ParseStockSignal` |
| Lógica upsert | `internal/scraping/upsert.go` — `ON CONFLICT` + price_history |
| Generador de SKU | `internal/scraping/sku.go` — `sha256(storeID:URL)[:16]` |
| Worker por tienda | `internal/scraping/worker.go` — UA rotation, paginación, anomaly check |
| Runner concurrente | `internal/scraping/runner.go` — semaphore=3, `RunAll`, `RunStore` |
| Notificador alertas | `internal/scraping/notifier.go` — Resend REST API |
| Scraper Sigmaelectrónica | `internal/scraping/stores/sigmaelectronica.go` + fixture + 5 tests |
| Scraper Electronilab | `internal/scraping/stores/electronilab.go` + fixture + 4 tests |
| Scraper Vistronica | `internal/scraping/stores/vistronica.go` + fixture + 4 tests (incluye "Consultar precio") |
| Tests unitarios | `parser_test.go` (22 casos), `sku_test.go` (4 casos) |
| Tests de integración | `integration_test.go` — full flow + anomaly (build tag: `integration`) |
| `cmd/scraper` completo | `cmd/scraper/main.go` — flags `--dry-run`, `--store`, signal handling |

### Pendiente
- **1.8** Scrapers adicionales: I+D Electrónica, JC Electrónica (Fase 1 ampliada)
- **1.10** Cloud Run Job + Cloud Scheduler deploy (requiere GCP configurado)

---

## Fase 2 — Catálogo + Frontend ✅

**Objetivo:** estudiante puede buscar y ver productos.

**Estado:** completa.

- API REST: búsqueda FTS, listado por categoría, detalle de listing (`internal/catalog`)
- Frontend Astro: homepage (SSG), búsqueda SSR, página de listing SSR, categorías SSR
- Timestamp "precio actualizado hace X" visible en cada listing (con alerta si >24h)

---

## Fase 3 — Usuarios + Carrito + Checkout ✅

**Objetivo:** estudiante puede crear cuenta, armar carrito y hacer un pedido.

**Estado:** completa.

- Auth: registro/login email+password, JWT HS256 24h (`internal/auth`, `internal/users`)
- Direcciones guardadas por usuario (`users/addresses`)
- Cart persistente en DB, migración de cart guest (`internal/cart`)
- Checkout: zona Bogotá/Soacha, tarifa por distancia, snapshots inmutables (`internal/orders`, `internal/delivery`)
- Email de confirmación al crear la orden via Resend (`internal/notifications`)
- Páginas Astro: register, login, cart, checkout

---

## Fase 4 — Panel del Operador + Notificaciones ✅

**Objetivo:** operador gestiona el ciclo de vida completo desde el celular.

**Estado:** completa.

### Lo implementado

| Tarea | Archivo(s) |
|-------|-----------|
| Auth de operadores | `internal/operator/auth_handler.go` — login/logout, cookie HttpOnly 8h, `RequireOperatorSession` middleware |
| Router del panel | `internal/operator/handler.go` — wires sub-handlers, separa rutas públicas y protegidas |
| Gestión de órdenes | `internal/operator/orders_handler.go` — listado paginado, detalle, confirm, transition, cancel item, override fee, assign courier, record payment |
| Máquina de estados | `internal/operator/transitions.go` — transiciones validadas con `FOR UPDATE` lock, eventos, notificaciones async |
| Gestión de mensajeros | `internal/operator/couriers_handler.go` — CRUD de couriers con patch parcial |
| Config de delivery | `internal/operator/delivery_config_handler.go` — lectura y reemplazo atómico de brackets + discount_pct |
| Logs de operación | `internal/operator/logs_handler.go` — notification_logs y scrape_jobs con filtros |
| Emails de estado | `internal/notifications/status_emails.go` — confirmed, in_delivery, delivered, cancelled, item_cancelled |
| Seed operador dev | `db/migrations/008_seed_operator.sql` — `operator@protou.co` / `operator123` |
| Frontend operador | `web/src/pages/operator/` — login, dashboard, detalle de orden, couriers, delivery-config |

---

## Fase 5 — Hardening ✅

**Objetivo:** sistema sólido antes de órdenes reales.

**Estado:** completa (código).

### Lo implementado

| Tarea | Detalle |
|-------|---------|
| HTTP Infrastructure | `http.Server` con timeouts, `SecurityHeaders` middleware, health check con DB ping |
| Rate Limiting | 10/min en `/v1/auth/register` y `/v1/auth/login`; 5/min en `/v1/operator/auth/login` |
| Input Validation | Email regexp, password mínimo 8 chars, `payment_method` enum, qty 1-99, `DELETE /v1/cart/items/{listing_id}` |
| HABEAS DATA | `009_habeas_data.sql`, `habeas_data_consent_at`, `AnonymizeUser`, `DELETE /v1/users/me`, página `/privacidad` |
| Tests E2E service layer | `internal/e2e/student_flow_test.go`, `internal/e2e/operator_flow_test.go` |
| Tests E2E HTTP layer | `internal/platform/integration_test.go` (TestStudentE2E, TestCheckoutOutOfZone), `internal/operator/integration_test.go` (TestOperatorE2E) |
| Unit tests handlers | `internal/users/handler_test.go` (INVALID_EMAIL, PASSWORD_TOO_SHORT), `internal/platform/httpserver_test.go` (SecurityHeaders) |
| Cloud Monitoring Alerts | `docs/infrastructure.md` — 3 alertas: high error rate, scraper failure, scraper yield drop |

### Pendiente (externo)
- Verificación de backups Cloud SQL
- Load test con k6
- Soft launch con contactos en universidades

**Done when:** todos los criterios de [mvp-criteria.md](./mvp-criteria.md) están marcados.
