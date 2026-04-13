# Spec: Fase 5 — Hardening

> Engram #113 | topic: `sdd/hardening/spec` | 2026-04-07

_Versión condensada. Ver engram ID #113 para el documento completo con todos los escenarios Given/When/Then._

---

## H1: HTTP Server Hardening

MUST usar `http.Server{}` con `ReadTimeout:10s`, `WriteTimeout:30s`, `IdleTimeout:120s`. MUST hacer graceful shutdown en SIGTERM (esperar hasta 30s). MUST rechazar body > 1MB con 413.

## H2: Rate Limiting en Auth

MUST usar `go-chi/httprate`. Student auth: máx 10 req/min por IP. Operator login: máx 5 req/min por IP. Retornar 429 con header `Retry-After`.

## H3: Security Headers

MUST aplicar en todas las respuestas:
- `X-Frame-Options: DENY`
- `X-Content-Type-Options: nosniff`
- `Strict-Transport-Security: max-age=63072000; includeSubDomains`
- `Content-Security-Policy-Report-Only: default-src 'self'`

## H4: Input Validation

| Campo | Regla | Error |
|-------|-------|-------|
| email (register) | Formato RFC5322 | 400 INVALID_EMAIL |
| password (register) | Mínimo 8 chars | 400 PASSWORD_TOO_SHORT |
| payment_method | Enum: nequi/daviplata/efectivo/llaves_breve | 400 INVALID_PAYMENT_METHOD |
| quantity (cart) | Entre 1 y 99 | 400 INVALID_QUANTITY |

## H5: Health Check con DB Ping

GET /health hace `db.PingContext(ctx, 3s)`.
- DB ok → 200 `{"status":"ok","db":"ok"}`
- DB down → 503 `{"status":"degraded","db":"error"}`

## H6: HABEAS DATA (Ley 1581/2012)

- `users` MUST tener columna `habeas_data_consent_at TIMESTAMPTZ`
- POST /register con `habeas_data: true` → columna poblada
- GET /privacidad MUST retornar 200 con política de privacidad en español
- DELETE /v1/users/me MUST retornar 202; anonimizar PII del usuario

## H7–H10: Testing + Logging

- `log/slog` JSON con `request_id` propagado desde chi middleware
- E2E integration tests (build tag `integration`): student flow + operator flow
- Coverage targets: cart >= 50%, orders >= 50%, operator >= 70%
