# Estado del proyecto

Última actualización: 2026-04-08

## Resumen

| Fase | Estado | Tests |
|------|--------|-------|
| 0 — Fundación | ✅ Completa (código) / ⏳ Setup externo | `go build ./...` OK |
| 1 — Scraper | ✅ Completa (código) / ⏳ Deploy | 29 tests pasando |
| 2 — Catálogo + Frontend | ✅ Completa | `go build ./...` OK |
| 3 — Usuarios + Carrito + Checkout | ✅ Completa | `go build ./...` OK |
| 4 — Panel del Operador | ✅ Completa | `go build ./...` OK |
| 5 — Hardening | ✅ Completa | Phases 1-4 completas |

---

## Fase 0 — Fundación ✅

### Completado
- [x] Go monorepo scaffold (`cmd/api`, `cmd/scraper`, `cmd/migrate`, `internal/*/`)
- [x] Dockerfile multi-target (`api`, `scraper`, `migrate`)
- [x] Migraciones goose — schema completo (20+ tablas, enums, índices, trigger FTS)
- [x] Seed delivery config (5 brackets de tarifa + discount_pct=30)
- [x] `internal/platform` — `config.go`, `db.go`, `httpserver.go` (chi + health)
- [x] `docker-compose.yml` — postgres+pgvector + API local
- [x] `.github/workflows/ci.yml` — lint + test (unit + integración) + build-check (solo PRs)
- [x] `.github/workflows/deploy-api.yml` — build+push → migrate → deploy Cloud Run → smoke test
- [x] `.github/workflows/deploy-scraper.yml` — build+push → actualiza Cloud Run Job
- [x] `docs/gcp-cicd-setup.md` — setup paso a paso: WIF + Artifact Registry + GitHub secrets
- [x] `Makefile` — `dev`, `dev-quick`, `dev-reset`, `build`, `test`, `lint`, `migrate-up/down`, `scraper-dry-run`
- [x] Astro scaffold en `web/` — SSR + Cloudflare adapter + Tailwind + `api.ts`
- [x] `.golangci.yml`, `.gitignore`, `.env.example`, `.env.dev.example`, `web/.env.example`
- [x] `web/tailwind.config.js` — config explícita para que el plugin escanee `src/**/*.{astro,...}`
- [x] `docker-compose.yml` reescrito — `db` → `db-sync` (pg_dump prod→dev) → `migrate` → `api`

### Pendiente (setup externo — acción del equipo)
- [ ] **0.1** GCP: crear proyecto, habilitar APIs (Cloud Run, Cloud Scheduler, Secret Manager, Artifact Registry), configurar IAM
- [x] **0.2** Neon: proyecto creado, `DATABASE_URL` configurado en `.env`, migraciones aplicadas (v13)
- [ ] **0.8** Cloudflare Pages: crear proyecto, conectar repo, configurar `PUBLIC_API_URL`

### Cómo levantar local

```bash
# 1. Variables de entorno
cp .env.example .env           # credenciales de producción (Neon, Resend, etc.)
cp .env.dev.example .env.dev   # overrides dev — llenar PROD_DATABASE_URL con el valor de DATABASE_URL de .env
cp web/.env.example web/.env   # PUBLIC_API_URL del frontend

# 2. Stack completo (DB → sync desde Neon → migrate → API)
make dev                       # levanta todo en Docker
curl localhost:8080/health     # {"status":"ok","db":"ok"}

# 3. Frontend
cd web && bun install && bun dev   # http://localhost:4321
```

**Variantes útiles:**
```bash
make dev-quick   # reinicia solo el API sin re-sincronizar (DB ya corriendo)
make dev-reset   # borra el volumen y sincroniza desde cero
```

---

## Fase 1 — Scraper ✅ (código) / ⏳ (deploy)

### Completado
- [x] Store discovery — `docs/stores-discovery.md` con 9 tiendas, selectores WooCommerce
- [x] Seed migrations — stores (3), scrape_rules (3), categorías (~52), tags (16)
- [x] `internal/scraping` package completo:
  - `types.go`, `repository.go`, `parser.go`, `upsert.go`, `sku.go`
  - `worker.go` — UA rotation, paginación, anomaly detection
  - `runner.go` — semaphore=3, RunAll, RunStore
  - `notifier.go` — Resend REST API + NoopNotifier
- [x] Scrapers con fixtures y tests:
  - Sigmaelectrónica (5 tests — strip IVA, "Agotado")
  - Electronilab (4 tests)
  - Vistronica (4 tests — "Consultar precio")
- [x] Tests unitarios: `parser_test.go` (22 casos), `sku_test.go` (4 casos)
- [x] Tests de integración: `integration_test.go` (build tag: `integration`)
- [x] `cmd/scraper/main.go` — flags `--dry-run`, `--store`, signal handling

### Pendiente
- [ ] **1.8** Scrapers adicionales: I+D Electrónica, JC Electrónica
- [ ] **1.10** Cloud Run Job + Cloud Scheduler deploy (requiere GCP configurado)

### Comandos útiles
```bash
# Tests unitarios del scraper
go test ./internal/scraping/... ./internal/scraping/stores/...

# Dry-run contra una tienda real
go run ./cmd/scraper --store=sigmaelectronica --dry-run

# Tests de integración (requiere TEST_DATABASE_URL)
make test-integration
```

---

## Fase 2 — Catálogo + Frontend ✅

### Completado
- [x] `internal/catalog/catalog.go` — tipos: `Listing`, `Category`, `Store`, `ListingFilters`, `Page`, `StockSignal`
- [x] `internal/catalog/repository.go` — queries FTS, listado por categoría, árbol de categorías
- [x] `internal/catalog/service.go` — `SearchListings`, `GetListing`, `ListCategoriesTree`, `GetCategoryBySlug`, `ListStores`
- [x] `internal/catalog/handler.go` — endpoints: `GET /v1/listings`, `GET /v1/listings/{id}`, `GET /v1/categories`, `GET /v1/categories/{slug}/listings`, `GET /v1/stores`
- [x] `web/src/pages/index.astro` — homepage SSG: hero con buscador, grid de categorías
- [x] `web/src/pages/search.astro` — búsqueda SSR: grid de resultados con paginación, `LastScrapedLabel` inline
- [x] `web/src/pages/listing/[id].astro` — detalle de listing SSR
- [x] `web/src/pages/categories/[slug].astro` — listado por categoría SSR
- [x] `cmd/api/main.go` — catalogo wired al router

---

## Fase 3 — Usuarios + Carrito + Checkout ✅

### Completado
- [x] `internal/auth/jwt.go` — `IssueToken`, `ValidateToken` (HS256, 24h)
- [x] `internal/auth/middleware.go` — `RequireStudent`, `RequireOperator`, `StudentFromContext`, `OperatorFromContext`
- [x] `internal/auth/session.go` — sesiones de operador en DB, `StartSessionCleanup` (goroutine cada 1h)
- [x] `internal/users/types.go` — `User`, `Address`, `AddressInput`, `RegisterRequest`
- [x] `internal/users/repository.go` — queries de usuarios y direcciones
- [x] `internal/users/service.go` — `Register` (bcrypt), `Login`, `GetUserByID`, `ListAddresses`, `AddAddress`, `DeleteAddress`
- [x] `internal/users/handler.go` — `POST /v1/auth/register`, `POST /v1/auth/login`, `GET /v1/users/me`, `DELETE /v1/users/me` (HABEAS DATA), `GET/POST/DELETE /v1/users/me/addresses`
- [x] `internal/cart/types.go` — `Cart`, `CartItem`, `CartItemEnriched`, `CartResponse`, `GuestItem`
- [x] `internal/cart/repository.go` — `GetCartWithItems`, `UpsertItem`, `ClearCart`
- [x] `internal/cart/service.go` — `GetCart` (enriquecido con listing data), `AddToCart`, `MigrateGuestCart`
- [x] `internal/cart/handler.go` — `GET /v1/cart`, `PUT /v1/cart/items/{listing_id}`, `DELETE /v1/cart/items/{listing_id}`, `DELETE /v1/cart`, `POST /v1/cart/migrate`
- [x] `internal/delivery/types.go` — `LatLng`, `FeeBracket`, `DeliveryConfig`, `Courier`, `FeeResult`
- [x] `internal/delivery/calculator.go` — cálculo de tarifa por distancia, descuento multi-tienda
- [x] `internal/delivery/repository.go` — brackets, config, couriers
- [x] `internal/orders/types.go` — `Order`, `OrderItem`, `OrderStatus`, `PaymentMethod`, `CreateOrderRequest`, `DeliveryFeeRequest`
- [x] `internal/orders/repository.go` — insert orden + items en una transacción, queries de lectura
- [x] `internal/orders/service.go` — `CreateOrder`, `CalculateDeliveryFee`, `ListOrders`, `GetOrder`
- [x] `internal/orders/handler.go` — `POST /v1/checkout/delivery-fee`, `POST /v1/orders`, `GET /v1/orders`, `GET /v1/orders/{id}`
- [x] `internal/notifications/email.go` — `SendOrderCreated` vía Resend REST API, template HTML, `notification_logs`
- [x] `web/src/pages/register.astro` — formulario de registro con HABEAS DATA consent
- [x] `web/src/pages/login.astro` — formulario de login + migración de carrito guest
- [x] `web/src/pages/cart.astro` — carrito SSR/CSR híbrido con qty controls
- [x] `web/src/pages/checkout.astro` — flujo 3-pasos: dirección → pago → confirmar
- [x] `cmd/api/main.go` — todos los dominios wired: catalog, users, cart, delivery, orders, notifications

---

## Fase 4 — Panel del Operador ✅

### Completado

- [x] `internal/operator/handler.go` — root handler, wires all sub-handlers, mounts public + protected route groups
- [x] `internal/operator/auth_handler.go` — `POST /v1/operator/auth/login` (bcrypt + HttpOnly session cookie 8h), `POST /v1/operator/auth/logout`, `RequireOperatorSession` middleware
- [x] `internal/operator/orders_handler.go` — `GET /v1/operator/orders` (paginado, filtrable por status), `GET /v1/operator/orders/{id}` (order + items + events + delivery), `POST /v1/operator/orders/{id}/confirm`, `POST /v1/operator/orders/{id}/transition`, `POST /v1/operator/orders/{id}/items/{item_id}/cancel` (recalcula totales, auto-cancela orden si todos los ítems cancelados), `PUT /v1/operator/orders/{id}/delivery-fee`, `POST /v1/operator/orders/{id}/assign-courier`, `POST /v1/operator/orders/{id}/payment`
- [x] `internal/operator/transitions.go` — máquina de estados (`validTransitions` map), `transitionOrderStatus` con `FOR UPDATE` lock, inserta `order_events`, dispara notificaciones async (fire-and-forget)
- [x] `internal/operator/couriers_handler.go` — `GET /v1/operator/couriers` (activos por default, `?all=true` incluye inactivos), `POST /v1/operator/couriers`, `PUT /v1/operator/couriers/{id}` (patch parcial con COALESCE)
- [x] `internal/operator/delivery_config_handler.go` — `GET /v1/operator/delivery-config`, `PUT /v1/operator/delivery-config` (reemplaza brackets atómicamente + upsert config)
- [x] `internal/operator/logs_handler.go` — `GET /v1/operator/notification-logs` (filtrable por `order_id`), `GET /v1/operator/scrape-jobs` (filtrable por `store_id`, limit máx 100)
- [x] `internal/notifications/status_emails.go` — `SendOrderConfirmed`, `SendOrderInDelivery`, `SendOrderDelivered`, `SendOrderCancelled`, `SendItemCancelled` — todos vía Resend REST API con log en `notification_logs`
- [x] `db/migrations/008_seed_operator.sql` — seed de usuario operador dev (`operator@protou.co` / `operator123`)
- [x] `cmd/api/main.go` — `operator.NewHandler(db, notifSvc)` wired al router principal
- [x] `web/src/pages/operator/login.astro` — página de login del operador
- [x] `web/src/pages/operator/index.astro` — dashboard principal (lista de órdenes agrupadas por estado)
- [x] `web/src/pages/operator/orders/[id].astro` — detalle de orden con acciones
- [x] `web/src/pages/operator/couriers.astro` — gestión de mensajeros
- [x] `web/src/pages/operator/delivery-config.astro` — configuración de tarifas y brackets

---

## Fase 5 — Hardening ✅

### Phase 1: HTTP Infrastructure
- [x] H1 — `http.Server` con timeouts (`ReadTimeout:10s`, `WriteTimeout:30s`, `IdleTimeout:120s`)
- [x] H3 — `SecurityHeaders` middleware (X-Frame-Options, X-Content-Type-Options, HSTS, CSP-Report-Only)
- [x] H5 — `healthHandler` con `db.PingContext` → `{"status":"ok","db":"ok"}` / 503 si DB no responde
- [x] H7 — `log/slog` con `JSONHandler`; `SlogMiddleware` propaga `request_id` en context; `LoggerFromContext` helper

### Phase 2: Rate Limiting + Input Validation
- [x] H2 — `httprate.LimitByIP` en `/v1/auth/register`, `/v1/auth/login` (10/min), `/v1/operator/auth/login` (5/min)
- [x] H4 — Validación de email (regexp), password mínimo 8 chars, `payment_method` enum, qty 1-99

### Phase 3: HABEAS DATA Compliance
- [x] H6 — Migración `009_habeas_data.sql`; `habeas_data_consent_at` en registro; `AnonymizeUser` + `DELETE /v1/users/me`; página `/privacidad`

### Phase 4: Testing + Observabilidad
- [x] 4.1 — `internal/e2e/student_flow_test.go` (build tag: `integration`) — flujo completo: register → login → add to cart → delivery fee
- [x] 4.2 — `internal/e2e/operator_flow_test.go` (build tag: `integration`) — operator login → list orders → transition
- [x] 4.3 — `docs/infrastructure.md` — sección "Cloud Monitoring Alerts" con instrucciones paso a paso para las 3 alertas
- [x] 4.4 — `docs/progress.md` — sección Fase 5 con checkboxes H1-H11
- [x] 4.5 — `docs/testing.md` — sección E2E integration tests
- [x] 4.6 — `internal/platform/httpserver_test.go` — unit tests para `SecurityHeaders` middleware
- [x] 4.7 — `internal/platform/integration_test.go` — integration tests HTTP: `TestStudentE2E`, `TestCheckoutOutOfZone`
- [x] 4.8 — `internal/operator/integration_test.go` — integration test HTTP: `TestOperatorE2E`
- [x] 4.9 — `internal/users/handler_test.go` — unit tests para validaciones de `/v1/auth/register` (INVALID_EMAIL, PASSWORD_TOO_SHORT)
- [x] 4.10 — `DELETE /v1/cart/items/{listing_id}` — endpoint para eliminar ítems individuales del carrito

---

## Decisiones pendientes del equipo

| # | Decisión | Impacto |
|---|----------|---------|
| 1 | ¿Cuándo configurar GCP (0.1)? | Desbloquea 1.10 y todos los deploys |
| 3 | ¿Scrapers de I+D y JC Electrónica antes o después de MVP? | Amplía el catálogo |
| 4 | Electronilab: ¿implementar cliente Algolia REST? | Actualmente desactivada (JS-rendered) |

---

## Árbol de archivos clave

```
protou/
├── cmd/
│   ├── api/main.go              ✅ wired: config → DB → server
│   ├── scraper/main.go          ✅ --dry-run, --store, signal handling
│   └── migrate/main.go          ✅ goose up embebido
├── db/
│   └── migrations/
│       ├── 001_extensions.sql   ✅ pgvector + pg_trgm
│       ├── 002_initial_schema.sql ✅ schema completo
│       ├── 003_seed_delivery_config.sql ✅
│       ├── 004_seed_stores.sql  ✅ 3 tiendas + scrape_rules
│       ├── 005_seed_categories.sql ✅ ~52 categorías
│       ├── 006_seed_tags.sql    ✅ 16 tags
│       ├── 008_seed_operator.sql ✅ seed operador dev
│       ├── 009_habeas_data.sql  ✅ habeas_data_consent_at + deleted_at
│       ├── 010_fix_scrape_rules.sql ✅ URL Sigma corregida, Electronilab desactivada
│       ├── 011_disable_vistronica.sql ✅ Vistronica desactivada (dominio inexistente)
│       ├── 012_fix_sigma_selectors.sql ✅ name→h3.kw-details-title, image→img.kw-prodimage-img-secondary
│       └── 013_fix_sigma_pagination.sql ✅ pagination→a.pagination-item-next-link
├── internal/
│   ├── platform/                ✅ config, db, httpserver
│   ├── scraping/                ✅ completo + 29 tests
│   │   └── stores/              ✅ sigma, electronilab, vistronica
│   ├── catalog/                 ✅ types, repository, service, handler
│   ├── orders/                  ✅ types, repository, service, handler
│   ├── cart/                    ✅ types, repository, service, handler
│   ├── delivery/                ✅ types, calculator, repository
│   ├── users/                   ✅ types, repository, service, handler, handler_test.go
│   ├── auth/                    ✅ jwt, middleware, session
│   ├── operator/                ✅ handler, auth, orders, transitions, couriers, delivery-config, logs, integration_test.go
│   ├── payments/                🔲 stub
│   └── notifications/           ✅ email (Resend), templates + status emails (Fase 4)
├── testdata/
│   ├── sigmaelectronica/        ✅ fixture HTML
│   ├── electronilab/            ✅ fixture HTML
│   └── vistronica/              ✅ fixture HTML
├── web/                         ✅ Astro — homepage, search, listing, categories, register, login, cart, checkout, operator panel
├── docs/                        ✅ documentación completa
├── Dockerfile                   ✅ multi-target
├── docker-compose.yml           ✅
├── .github/workflows/
│   ├── ci.yml                   ✅ lint + test + build-check
│   ├── deploy-api.yml           ✅ WIF → build → migrate → deploy → smoke test
│   └── deploy-scraper.yml       ✅ WIF → build → update Cloud Run Job
└── Makefile                     ✅
```
