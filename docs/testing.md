# Testing

## Overview

El proyecto prioriza tests de **lógica de negocio pura** sobre cobertura exhaustiva de infraestructura. La filosofía es:

- Las reglas de dominio (cálculo de delivery, transiciones de estado, parsing de precios) se testean con tablas de casos exhaustivas.
- La capa de servicio se testea a nivel de predicado/contrato cuando la lógica es extraíble sin DB.
- Los integration tests corren contra una DB real y se activan solo con build tag `integration`.
- Los handlers HTTP se testean en CI mediante el smoke test de build; cobertura profunda no es prioritaria en Fase 1.

---

## Estructura de tests

```
internal/
  delivery/
    calculator_test.go     # unit — tabla de casos para Calculate e IsAddressCovered
  scraping/
    parser_test.go         # unit — ParsePrice, ParseStockSignal
    integration_test.go    # integration (//go:build integration) — RunStore + DB real
  cart/
    service_test.go        # unit — predicados de IsActive, GuestItem, CartResponse
  users/
    handler_test.go        # unit (!integration) — validación de register (INVALID_EMAIL, PASSWORD_TOO_SHORT)
                           #   usa go-sqlmock; no requiere DB real
  operator/
    transitions_test.go    # unit — máquina de estados isTransitionAllowed
    delivery_config_test.go # unit — validación de delivery config
    integration_test.go    # integration (//go:build integration) — TestOperatorE2E:
                           #   operator login → list orders → confirm → transition → payment
  platform/
    httpserver_test.go     # unit (!integration) — SecurityHeaders middleware (4 tests)
    integration_test.go    # integration (//go:build integration) — TestStudentE2E, TestCheckoutOutOfZone
                           #   flujo HTTP completo contra DB real: register → login → cart → checkout
  e2e/
    student_flow_test.go   # integration (//go:build integration) — service layer directo (sin HTTP)
                           #   register → login → cart → delivery fee, duplicate email
    operator_flow_test.go  # integration (//go:build integration) — service layer directo (sin HTTP)
                           #   operator login → list orders → transition
```

Los fixtures HTML para integration tests se definen como constantes en el mismo archivo (`testFixtureHTML`, `testFixtureHTMLUpdatedPrice`). No hay directorio `testdata/` por ahora; si un fixture crece, moverlo a `internal/<pkg>/testdata/`.

---

## Build tags

| Tag | Cuándo usar |
|-----|-------------|
| `//go:build integration` | Tests que requieren DB real (`TEST_DATABASE_URL`) |
| `//go:build !integration` | Tests que NO deben correr con `-tags integration` (evita conflictos de package) |

**Regla**: todo archivo de test que abre una conexión a Postgres lleva `//go:build integration` en la primera línea. Los unit tests que operan en el mismo package (no `_test`) y podrían colisionar con integration tests llevan `//go:build !integration`.

Nombre de archivo: `integration_test.go` para tests de integración (convención del proyecto).

---

## Correr los tests

### Unit tests solamente (sin DB)
```bash
make test
# equivalente a:
go test -race -timeout 120s ./...
```

### Integration tests del scraper (requieren DB)
```bash
export TEST_DATABASE_URL=postgres://postgres:password@localhost:5432/protou?sslmode=disable
make test-integration
# equivalente a:
go test -tags integration -v -timeout 120s ./internal/scraping/... ./internal/scraping/stores/...
```

### E2E integration tests — service layer (sin HTTP)

Los tests en `internal/e2e/` ejercen las capas de service y repository directamente contra una DB real. No requieren el servidor levantado. Usan `TEST_DATABASE_URL`.

```bash
export TEST_DATABASE_URL=postgres://postgres:password@localhost:5432/protou?sslmode=disable

# Student flow: register → login → cart → delivery fee
go test -tags integration -v -timeout 120s ./internal/e2e/... -run TestStudentFlow

# Operator flow: login → list orders → transition
go test -tags integration -v -timeout 120s ./internal/e2e/... -run TestOperatorFlow

# Todos los E2E tests de service layer
go test -tags integration -v -timeout 120s ./internal/e2e/...
```

Si `TEST_DATABASE_URL` no está definida, los tests hacen `t.Skip` automáticamente.

### E2E integration tests — HTTP layer

Los tests en `internal/platform/integration_test.go` ejercen el stack HTTP completo (handlers + services + repositories) contra una DB real usando `httptest.NewServer`. Usan `DATABASE_URL`.

```bash
export DATABASE_URL=postgres://postgres:password@localhost:5432/protou?sslmode=disable

# Student flow HTTP: register → login → add to cart → checkout
go test -tags integration -v -timeout 120s ./internal/platform/... -run TestStudentE2E

# Checkout fuera de zona
go test -tags integration -v -timeout 120s ./internal/platform/... -run TestCheckoutOutOfZone

# Operator E2E HTTP (en internal/operator/)
go test -tags integration -v -timeout 120s ./internal/operator/... -run TestOperatorE2E

# Todos los integration tests HTTP
go test -tags integration -v -timeout 120s ./internal/platform/... ./internal/operator/...
```

> `TEST_DATABASE_URL` es para los E2E de service layer (`internal/e2e/`). `DATABASE_URL` es para los integration tests HTTP (`internal/platform/` e `internal/operator/`). Ambas pueden apuntar a la misma instancia local.

### Package específico
```bash
go test -race ./internal/delivery/...
go test -race -run TestParsePrice ./internal/scraping/...
```

### Con coverage
```bash
go test -race -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### CI vs local

| Contexto | Comando | DB disponible |
|----------|---------|---------------|
| CI (GitHub Actions) | `go test -race ./...` + `go test -tags integration ./internal/scraping/...` | Sí (servicio PostgreSQL en el job) |
| Local (docker compose) | `make test` / `make test-integration` | Sí si `docker compose up` está corriendo |
| Local (sin docker) | `make test` | Solo unit tests |

En CI las variables de entorno se configuran automáticamente (ver `.github/workflows/ci.yml`, job `test`).

---

## Targets de cobertura

| Package | Target | Justificación |
|---------|--------|---------------|
| `internal/delivery` | >= 70% | Lógica pura: cálculo de fee, haversine, cobertura geográfica |
| `internal/operator` | >= 70% | Máquina de estados crítica para el flujo de órdenes |
| `internal/scraping` (parser) | >= 60% | ParsePrice y ParseStockSignal cubren edge cases de scraping |
| `internal/cart` | >= 50% | Service layer; parte de la lógica depende de DB |
| `internal/catalog` | >= 50% | Service layer |
| `internal/orders` | >= 50% | Service layer |
| `internal/handler` | >= 30% | HTTP handlers; prioridad baja en Fase 1 |

---

## Patrones de testing

### Table-driven tests

Patrón estándar del proyecto. Ver `internal/scraping/parser_test.go` (`TestParsePrice`) e `internal/operator/transitions_test.go` (`TestIsTransitionAllowed`):

```go
cases := []struct {
    input    string
    expected int
    ok       bool
}{
    {"$ 1.200", 1200, true},
    {"Consultar precio", 0, false},
}
for _, tc := range cases {
    tc := tc
    t.Run(tc.input, func(t *testing.T) {
        got, ok := ParsePrice(tc.input)
        assert.Equal(t, tc.ok, ok)
    })
}
```

### Unit tests sin mock de repositorio

Cuando la lógica del service es extraíble (predicados, validaciones), se testea directamente sobre los tipos de dominio sin instanciar el service. Ver `internal/cart/service_test.go`:

```go
func TestListingIsActive_OutOfStock_CartRejectsIt(t *testing.T) {
    l := &catalog.Listing{StockSignal: catalog.StockOut}
    assert.False(t, l.IsActive())
}
```

### Integration tests con DB real

Patrón en `internal/scraping/integration_test.go`:

1. `openTestDB(t)` — skip automático si `TEST_DATABASE_URL` no está definido.
2. `insertTestStore` / `cleanupStore` — setup y teardown con `t.Cleanup` o `defer`.
3. `httptest.NewServer` — servidor HTTP en memoria para simular páginas scrapeadas.
4. Assertions sobre estado en DB después de cada operación.

```go
//go:build integration

func TestRunStore_FullFlow(t *testing.T) {
    db := openTestDB(t)
    defer db.Close()

    srv := serveFixturePage(testFixtureHTML)
    defer srv.Close()

    storeID := insertTestStore(t, db, "Test Store", srv.URL, srv.URL+"/?page={page}")
    defer cleanupStore(t, db, storeID)

    // ... assertions contra DB
}
```

### Notifiers/colaboradores de test

Para capturar llamadas a interfaces externas (notificaciones, emails), definir un tipo local en el archivo de test:

```go
type captureNotifier struct {
    onAlert func(storeName string, found, historical int)
}
func (n *captureNotifier) SendScraperAlert(...) error { ... }
```

---

## Qué NO testar

- **DTOs y structs de request/response** — campos y tipos son suficientes.
- **Constantes y enums** — a menos que haya lógica de validación.
- **Handlers triviales** — los que solo deserializan y llaman a un service (cobertura >= 30% cubre el caso de error básico).
- **Código generado por sqlc** — no es código propio.
- **Migraciones SQL** — se validan en CI al levantar la DB de test.

---

## Checklist para PRs

- [ ] Los unit tests nuevos no tienen dependencia de DB (sin `//go:build integration`).
- [ ] Los integration tests nuevos tienen `//go:build integration` y usan `openTestDB(t)` (skip si no hay DB).
- [ ] `make test` pasa localmente sin `TEST_DATABASE_URL`.
- [ ] Los targets de cobertura del package afectado se mantienen o mejoran.
- [ ] Cada test tiene un nombre descriptivo con `t.Run(...)`.
- [ ] No hay `t.Skip` permanentes — los skips deben ser condicionales a una env var ausente.
