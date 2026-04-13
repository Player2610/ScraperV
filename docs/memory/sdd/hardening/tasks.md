# Tasks: Fase 5 — Hardening

> Engram #115 | topic: `sdd/hardening/tasks` | 2026-04-07

---

## Phase 1: HTTP Infrastructure

- [x] 1.1 `cmd/api/main.go`: `http.Server` con ReadTimeout:10s, WriteTimeout:30s, IdleTimeout:120s
- [x] 1.2 `cmd/api/main.go`: signal handler SIGTERM + `srv.Shutdown(ctx 30s)` + `db.Close()`
- [x] 1.3 `cmd/api/main.go`: `slog.NewJSONHandler` + `slog.SetDefault`
- [x] 1.4 `internal/platform/httpserver.go`: `NewServer(cfg Config, db *sql.DB)`
- [x] 1.5 SecurityHeaders middleware (X-Frame-Options, X-Content-Type-Options, HSTS, CSP-Report-Only)
- [x] 1.6 `SlogMiddleware` + `LoggerFromContext(ctx) *slog.Logger`
- [x] 1.7 `healthHandler` con `db.PingContext` → 200/503
- [x] 1.8 Migrar `log.Printf` → `slog` en orders, operator

## Phase 2: Rate Limiting + Input Validation

- [x] 2.1 `go.mod`: agregar `go-chi/httprate`
- [x] 2.2 Rate limiting en `/v1/auth/register` y `/v1/auth/login` (10/min)
- [x] 2.3 Rate limiting en `/v1/operator/auth/login` (5/min)
- [x] 2.4 Validar email (INVALID_EMAIL) + password min 8 chars (PASSWORD_TOO_SHORT)
- [x] 2.5 Validar `payment_method` enum (INVALID_PAYMENT_METHOD)
- [x] 2.6 Validar `quantity` 1–99 (INVALID_QUANTITY) + MaxBytesReader

## Phase 3: HABEAS DATA

- [x] 3.1 `db/migrations/009_habeas_data.sql`
- [x] 3.2 `users/types.go`: HabeasDataConsent + HabeasDataConsentAt + DeletedAt
- [x] 3.3 `users/repository.go`: CreateUser con consentAt + AnonymizeUser
- [x] 3.4 `users/service.go`: propagar consent + AnonymizeUser
- [x] 3.5 `users/handler.go`: leer consent en register + deleteAccountHandler
- [x] 3.6 Registrar `DELETE /v1/users/me` en NewServer
- [x] 3.7 `web/src/pages/privacidad.astro`

## Phase 4: Testing + Observability

- [x] 4.1 `internal/platform/integration_test.go` — TestStudentE2E (build:integration)
- [x] 4.2 TestCheckoutOutOfZone en mismo archivo
- [x] 4.3 `internal/operator/integration_test.go` — TestOperatorE2E (build:integration)
- [x] 4.4 TestSecurityHeaders en httpserver_test.go
- [x] 4.5 `internal/users/handler_test.go` — TestRegisterValidation
- [x] 4.6 `docs/infrastructure.md` — sección "Alertas Cloud Monitoring"
