# Proposal: Fase 5 — Hardening

> Engram #112 | topic: `sdd/hardening/proposal` | 2026-04-07

## Intent

El sistema funcional (Fases 0–4) nunca ha recibido tráfico real. Antes de cualquier soft launch es necesario cerrar las brechas de seguridad, cumplimiento legal y observabilidad que lo exponen a ataques triviales (brute force, slow-loris), pérdida silenciosa de requests en deploys, y riesgo legal bajo Ley 1581/2012 (HABEAS DATA).

## Scope

### In Scope
- H1: HTTP server con timeouts + graceful shutdown + MaxBytesReader
- H2: Rate limiting en auth endpoints (go-chi/httprate)
- H3: Security headers middleware
- H4: Input validation: email, password, PaymentMethod enum, cart quantity
- H5: Health check con db.PingContext
- H6: HABEAS DATA — habeas_data_consent_at + /privacidad + DELETE /v1/users/me
- H7: Structured logging con log/slog JSON
- H8–H10: E2E + service + operator tests
- H11–H13: Cloud Monitoring alerts, k6, backup drill

### Out of Scope
- JWT refresh tokens / token revocation
- Repositorios a interfaces para mocking (deuda técnica post-MVP)
- internal/payments (stub intencional)

## Rollback Plan
- H1–H5, H7: cambios de middleware/config — revert commit en < 2 min
- H6: migración additive (columna nullable) — revert código; columna queda sin downtime
- H8–H10: solo tests, sin impacto en producción

## Success Criteria (clave)
- `GET /health` pinga DB real
- 11 requests a auth → 429
- Headers X-Frame-Options, HSTS presentes en todas las respuestas
- Tabla users tiene `habeas_data_consent_at`
- `DELETE /v1/users/me` retorna 202
- E2E tests pasan: student + operator flows
