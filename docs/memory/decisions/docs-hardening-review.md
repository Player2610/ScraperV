# Discovery: Docs Review — Hardening Complete

> Engram #119 | 2026-04-08 | topic: `docs/hardening-review`

**What**: Revisión completa de la documentación del proyecto tras completar Fase 5 Hardening.

**Where**: `docs/api.md`, `docs/testing.md`, `docs/progress.md`, `docs/phases.md`

**Desactualizaciones encontradas y corregidas:**

1. **api.md — endpoints faltantes**:
   - `DELETE /v1/cart/items/{listing_id}` no estaba documentado (idempotente, 204)
   - `DELETE /v1/users/me` no estaba documentado (HABEAS DATA, retorna 202)
   - `GET /health` no estaba documentado

2. **api.md — comportamiento incorrecto**:
   - `PUT /v1/cart/items/{listing_id}` decía "quantity=0 elimina el ítem" — INCORRECTO. El código valida quantity 1-99. Para eliminar hay que usar DELETE.

3. **api.md — validaciones faltantes en register**:
   - 400 `INVALID_EMAIL` (formato de email)
   - 400 `PASSWORD_TOO_SHORT` (< 8 chars)
   - Campo `habeas_data_consent?` en el body

4. **api.md — validaciones faltantes en POST /orders**:
   - 400 `MISSING_FIELDS` (payment_method ausente)
   - 400 `INVALID_PAYMENT_METHOD` (valor no reconocido)

5. **api.md — rate limiting no documentado**:
   - 429 `RATE_LIMITED`: 10/min en auth de estudiante, 5/min en operator login

6. **api.md — health check**: el código retorna 503 (no 200) con `{"status":"degraded","db":"error"}` en caso de fallo.

7. **testing.md — estructura de tests incompleta**: faltaban handler_test.go de cart, orders, operator integration_test, platform integration_test.

8. **progress.md**: Fase 5 marcada como "En progreso" cuando está completa.

9. **phases.md**: Fase 5 no tenía tabla de lo implementado.
