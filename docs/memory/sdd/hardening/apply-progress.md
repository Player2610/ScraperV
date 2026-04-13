# Apply Progress: Fase 5 — Hardening

> Engram #116 | topic: `sdd/hardening/apply-progress` | Completed: 2026-04-07

## Status: COMPLETE

---

## Phase 1: HTTP Infrastructure

- [x] 1.1–1.4: http.Server timeouts, graceful shutdown, slog JSON, NewServer(*sql.DB) — ya estaban
- [x] 1.5: SecurityHeaders — ya existía; ajustado HSTS max-age 1yr→2yr (63072000), siempre se envía
- [x] 1.6: LoggerFromContext(ctx) helper exportado — agregado
- [x] 1.7: health handler → `{"status":"ok","db":"ok"}` (200) / `{"status":"degraded","db":"error","error":"..."}` (503)
- [x] 1.8: slog migration — ya estaba hecho

## Phase 2: Rate Limiting + Input Validation

- [x] 2.1–2.3: httprate, LimitByIP — ya estaban
- [x] 2.4: email/password errors → `{"error":"INVALID_EMAIL"}` / `{"error":"PASSWORD_TOO_SHORT"}`
- [x] 2.5: payment_method error → `{"error":"INVALID_PAYMENT_METHOD"}`
- [x] 2.6: quantity 1–99 con `{"error":"INVALID_QUANTITY"}` + MaxBytesReader en upsertItem

## Phase 3: HABEAS DATA — COMPLETE (sesión anterior)

- [x] 3.1–3.7: migration 009, user fields, AnonymizeUser, DELETE /v1/users/me, privacidad.astro

## Phase 4: Testing + Observability

- [x] 4.1: internal/platform/integration_test.go — TestStudentE2E
- [x] 4.2: TestCheckoutOutOfZone (São Paulo → 422 OUTSIDE_ZONE, sin orden)
- [x] 4.3: internal/operator/integration_test.go — TestOperatorE2E
- [x] 4.4: TestSecurityHeaders en httpserver_test.go — 5 variantes
- [x] 4.5: internal/users/handler_test.go — TestRegisterValidation (3 casos)
- [x] 4.6: docs/infrastructure.md — sección "Alertas Cloud Monitoring"

## Gotchas

- `quantity=0` antes podía usarse para remover un item del carrito. Ahora bloqueado (1-99). Solución: `DELETE /v1/cart/items/{listing_id}` agregado.
- TestSecurityHeaders existente tenía aserción incorrecta (HSTS ausente en HTTP) — corregido.

## Files Modified

- `internal/platform/httpserver.go` — HSTS 2yr, LoggerFromContext, health format
- `internal/users/handler.go` — error codes consistentes
- `internal/orders/handler.go` — error code consistente
- `internal/cart/handler.go` — quantity 1-99, MaxBytesReader
- `internal/platform/httpserver_test.go` — TestSecurityHeaders + variantes
- `internal/platform/integration_test.go` — nuevo (build:integration)
- `internal/operator/integration_test.go` — nuevo (build:integration)
- `internal/users/handler_test.go` — nuevo (TestRegisterValidation)
- `docs/infrastructure.md` — sección alertas GCP
