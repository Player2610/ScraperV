# Exploración: Fase 5 — Hardening

> Engram #111 | topic: `sdd/hardening/explore` | 2026-04-07

## Current State

Fases 0–4 completan el sistema funcional. La API corre en Go con chi, PostgreSQL, JWT para estudiantes y sesiones cookie para operadores. 82 tests unitarios existen pero con coverage muy heterogéneo. El sistema nunca ha recibido tráfico real.

---

## Hallazgos por área

### 1. Testing Gaps (crítico)

- `cart.Service.AddToCart()` — no hay tests de integración
- `orders.Service.CreateOrder()` — 0% coverage del service layer
- `operator/auth_handler.go` — solo testea validaciones (5.3%)
- No hay E2E tests de flujo completo
- Targets: cart >= 50%, orders >= 50%, operator >= 70% (no alcanzados)

### 2. Seguridad (crítico)

- **Rate limiting: AUSENTE** en todos los endpoints de auth
- **Security headers: AUSENTES** (X-Frame-Options, X-Content-Type-Options, HSTS, CSP)
- **HTTP server timeouts: AUSENTES** — usa `http.ListenAndServe()` directo
- **Body size limits: AUSENTES** en todos los handlers
- **Input validation incompleta**: PaymentMethod, cart quantity, email format, password mínimo

### 3. Logging (medio)

- Todo usa `log.Printf()` — no estructurado, no correlacionable con request IDs

### 4. Observabilidad (medio)

- `/health` retorna `{"status":"ok"}` fijo — no verifica DB

### 5. Legal / Compliance (crítico)

- HABEAS DATA: checkbox en frontend pero sin persistencia en DB
- `/privacidad` page no existe
- Sin endpoint `DELETE /v1/users/me`

---

## Hardening Task Breakdown

| Grupo | Items | Prioridad |
|-------|-------|-----------|
| A: Security | H1–H6 | CRÍTICO — bloqueante MVP |
| B: Testing | H7–H10 | IMPORTANTE — pre-launch |
| C: Operacional | H11–H13 | Post soft launch |
