# Discovery: Test and Doc Review Findings (pre-Fase 5)

> Engram #109 | 2026-04-07 | topic: `review/tests-and-docs`

**What**: Revisión exhaustiva de tests y documentación del proyecto (Fases 0–4 completas).

**Why**: Pre-Fase 5 (Hardening) — identificar gaps antes de ir a producción.

**Where**: Todo el proyecto — `internal/*/`, `docs/`, `web/`

**Learned**:
- 82 tests unitarios totales, pero coverage MUY heterogénea
- CRÍTICO: `cart.Service.AddToCart()` y `orders.Service.CreateOrder()` tienen 0% coverage — paths de negocio más importantes
- `operator/auth_handler_test.go` solo testea validación (5.3%), no happy paths
- `orders/service_test.go` es básicamente 0% service coverage
- `docs/api.md` le faltaban 4 endpoints del operador: `items/{id}/cancel`, `delivery-fee PUT`, `assign-courier`, `payment`
- `docs/delivery.md` tenía el algoritmo multi-tienda INCORRECTO: decía "inter_dist_km × 1000" pero código usa "baseFee × 1.10" (10% surcharge fijo)
- Falta completamente `docs/testing.md` (estrategia de testing)
- 11/16 docs están bien; los 5 con problemas son: api.md, delivery.md, progress.md + falta testing.md

**Top 5 prioridades identificadas:**
1. Integration tests `cart.Service.AddToCart` + `orders.Service.CreateOrder`
2. Actualizar api.md con endpoints faltantes
3. Corregir algoritmo en delivery.md
4. Crear docs/testing.md
5. Handler HTTP tests operador (auth + transitions)
