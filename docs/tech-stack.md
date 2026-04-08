# Tech Stack

## Backend (Go)

| Librería | Uso |
|----------|-----|
| `go-chi/chi v5` | Router HTTP — idiomatic Go, stdlib-compatible, middleware limpio |
| `go-chi/cors` | CORS middleware |
| `go-chi/httprate` | Rate limiting por IP (register, login) |
| `golang-jwt/jwt v5` | JWT para auth de estudiantes (HS256, 24h) |
| `pressly/goose v3` | Migraciones SQL — archivos `.sql` legibles, up/down, embeddable en binario |
| `lib/pq` | Driver PostgreSQL para `database/sql` |
| `PuerkitoBio/goquery` | Parser HTML para el scraper (CSS selectors) |
| `golang.org/x/crypto` | bcrypt para passwords |
| `stretchr/testify` | Assertions en tests unitarios |
| `DATA-DOG/go-sqlmock` | Mock de `database/sql` para unit tests de handlers |

> Resend se consume vía REST API directo (`net/http`), sin SDK. Google Maps API igual. No se usa ORM ni sqlc.

## Frontend (Astro)

| Paquete | Uso |
|---------|-----|
| `astro` | Framework — SSR + SSG |
| `@astrojs/cloudflare` | Adapter para deploy en Cloudflare Pages |
| `tailwindcss` | Estilos |
| `typescript` | Strict mode |

## Infraestructura

| Herramienta | Dev | Prod |
|-------------|-----|------|
| Docker Compose | API + DB (pgvector) local | — |
| Neon | Fuente del sync (PROD_DATABASE_URL, solo lectura) | PostgreSQL serverless |
| Cloud SQL | — | PostgreSQL 16 + pgvector (~$9/mes) |
| GCP Cloud Run | — | API |
| GCP Cloud Run Job | — | Scraper nightly |
| GCP Cloud Scheduler | — | Trigger cron del scraper |
| GCP Secret Manager | — | Secrets en runtime |
| GCP Artifact Registry | — | Registro de imágenes Docker |
| Cloudflare Pages | — | Frontend (CDN global) |
| GitHub Actions | CI (lint, test, build-check) | Deploy (build → migrate → deploy) |

## Testing

| Herramienta | Uso |
|-------------|-----|
| `go test` con `-race` | Tests unitarios y de integración |
| `go-sqlmock` | Mock de DB para unit tests sin levantar Postgres |
| `net/http/httptest` | Tests E2E de handlers HTTP |
| `build tag: integration` | Tests de integración que requieren `TEST_DATABASE_URL` real |

## Herramientas de desarrollo

| Herramienta | Uso |
|-------------|-----|
| `golangci-lint` | Linter Go (configurado en `.golangci.yml`) |
| `goose` | CLI de migraciones |
| `make` | Comandos: `dev`, `dev-quick`, `dev-reset`, `test`, `lint`, `migrate-up/down`, `scraper-dry-run` |

## Convenciones de código

- **Sin ORM ni sqlc**: todas las queries en `internal/<domain>/repository.go` escritas a mano con `database/sql`
- **Dependency injection**: por constructor, sin globales
- **Dirección de dependencias**: `cmd/*` → `internal/*` → `internal/platform`. Los dominios no se importan entre sí excepto los casos definidos en `domains.md`
- **Funciones puras para lógica de negocio crítica**: `DeliveryFeeCalculator.Calculate()` no tiene efectos secundarios — fácil de unit-testear
- **Migraciones aditivas**: en MVP no se usa `DROP TABLE` ni `DROP COLUMN` — todo se agrega, nada se quita
