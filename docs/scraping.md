# Arquitectura de scraping

## Overview

El scraper es un binario Go separado (`cmd/scraper`) que corre como Cloud Run Job disparado por Cloud Scheduler. Comparte código de dominio con el API pero es un proceso completamente independiente.

## Estructura del código

```
internal/scraping/
├── types.go          — Store, ScrapeRule, StoreWithRule, ScrapeJob, RawListing, StockSignal
├── repository.go     — LoadActiveStoresWithRules, CreateJob, UpdateJob, GetLastSuccessfulJobCount
├── parser.go         — StoreParser interface, DefaultParser (goquery), ParsePrice, ParseStockSignal
├── upsert.go         — UpsertListing: INSERT ON CONFLICT + price_history on change
├── sku.go            — GenerateSKU: sha256(storeID:productURL)[:16]
├── worker.go         — Worker: UA rotation, paginación, anomaly check, job lifecycle
├── runner.go         — Runner: RunAll (semaphore=3), RunStore
├── notifier.go       — Notifier interface, EmailNotifier (Resend), NoopNotifier
├── goquery_util.go   — helpers internos
├── *_test.go         — 26 tests unitarios
├── integration_test.go — tests de integración (build tag: integration)
└── stores/
    ├── sigmaelectronica.go + test
    ├── electronilab.go + test
    └── vistronica.go + test

testdata/
├── sigmaelectronica/catalog_page.html
├── electronilab/catalog_page.html
└── vistronica/catalog_page.html
```

## Cuándo corre

Nightly a las 02:00 Colombia (UTC-5) vía Cloud Scheduler → Cloud Run Job.
Un goroutine por tienda, semaphore de concurrencia = 3.

## Flujo de un job

```
1. LoadActiveStoresWithRules(ctx, db) — carga tiendas activas con sus reglas desde DB
2. Para cada tienda (goroutine con semaphore=3):
   a. CreateJob → INSERT scrape_job (status=running)
   b. Fetch páginas con delay_ms + UA rotativo
   c. DefaultParser.Parse(html, rule) → []RawListing
   d. UpsertListing por cada raw listing
   e. checkAnomaly: si found < 20% histórico → alerta + status=partial
   f. UpdateJob → status=success|partial|failed
3. Errores por tienda no bloquean otras tiendas
```

## Parseo de precio

`ParsePrice(raw string) (int, StockSignal)`:
- Strip "COP", "$", ".", espacios → parsear entero
- Si contiene "consultar" → `price_on_request`, retorna 0
- Si precio = 0 o no parseable → `price_on_request`
- Precio en rango ("$1.000 – $2.000") → tomar primer valor

## Parseo de stock

`ParseStockSignal(priceRaw, stockRaw string) StockSignal`:
- `price_on_request` si precio no parseable o texto "consultar"
- `out_of_stock` si stockRaw contiene "agotado", "sin stock", "out of stock"
- `in_stock` si stockRaw contiene "disponible", "en stock"
- `unknown` en cualquier otro caso

## Generación de SKU

Si la tienda no expone un SKU explícito:
```go
GenerateSKU(storeID int64, productURL string) string
// sha256(fmt.Sprintf("%d:%s", storeID, productURL)) → primeros 16 hex chars
// Determinístico: misma URL → mismo SKU en todas las ejecuciones
```

## Upsert de listings

```
SELECT listing WHERE (store_id, external_sku) = (?, ?)
IF NOT EXISTS:
  INSERT → isNew=true
ELSE IF price_cop o stock_signal cambiaron:
  UPDATE → isUpdated=true
  INSERT INTO price_history
ELSE:
  no write → sin cambio
```

La deduplicación usa `UNIQUE (store_id, external_sku)`.

## ScrapeRules en DB

Los selectores CSS viven en la tabla `scrape_rules` — editables desde el panel sin redeploy.
Ver `db/migrations/004_seed_stores.sql` para los selectores reales de cada tienda.

## Selectores por plataforma

### WooCommerce estándar (I+D Electrónica, JC Electrónica, futuras tiendas)

| Campo | Selector CSS |
|-------|-------------|
| Item | `ul.products li.product` |
| Nombre | `h2.woocommerce-loop-product__title` |
| Precio | `span.woocommerce-Price-amount bdi` |
| Imagen | `img.wp-post-image` |
| URL | `a.woocommerce-LoopProduct-link[href]` |
| Paginación | `a.next.page-numbers` |
| Agotado | `.button.disabled`, `.out-of-stock`, texto "Agotado" |

### Sigmaelectrónica (tema Kallyas — selectores custom)

| Campo | Selector CSS | Nota |
|-------|-------------|------|
| Item | `ul.products li.product` | — |
| Nombre | `h3.kw-details-title` | No usa `h2.woocommerce-loop-product__title` |
| Precio | `span.woocommerce-Price-amount bdi` | Precio en COP con coma decimal |
| Imagen | `img.kw-prodimage-img-secondary` | La imagen primaria usa `data-echo` (lazy); la secundaria tiene `src` |
| URL | `a.woocommerce-LoopProduct-link[href]` | — |
| Paginación | `a.pagination-item-next-link` | No usa `a.next.page-numbers` |
| Agotado | `.out-of-stock, .button.disabled` | — |

## Manejo de datos faltantes

| Caso | Tratamiento |
|------|-------------|
| Sin precio visible | `stock_signal: price_on_request` — listing no mostrado al usuario |
| "Consultar precio" | `stock_signal: price_on_request`, `price_cop: NULL` |
| Stock no determinable | `stock_signal: unknown` — mostrado con advertencia |
| Sin imagen | Placeholder de categoría |
| Precio en USD | `stock_signal: price_on_request` — sin conversión en MVP |
| Sin SKU en tienda | `GenerateSKU(storeID, productURL)` |

## Anti-bot

- Pool de 5 User-Agents reales (rotativos) en el Worker (producción)
- El **dry-run** también usa UA de Chrome real — user-agents de bot causan que las tiendas sirvan HTML vacío
- `delay_ms` configurable por tienda (default 2000ms)
- Respeto de `robots.txt` donde aplique
- Sin proxies en MVP
- Tiendas con JS rendering (ej: Electronilab/Algolia) requieren cliente API dedicado, no goquery

## Detección de scraper roto

| Condición | Acción |
|-----------|--------|
| `listings_found == 0` | `status=failed` + email alerta inmediata |
| `found < 20% del histórico` | `status=partial` + email alerta |
| Error HTTP / timeout | `status=failed` + email alerta con mensaje |
| Normal (≥ 20%) | `status=success` — sin alerta |

El email incluye: nombre de tienda, job_id, found, baseline histórico, error.

## Tiendas implementadas

Ver tabla completa y selectores en [stores-discovery.md](./stores-discovery.md).

| Tienda | Estado en DB | Razón | Archivo parser |
|--------|-------------|-------|----------------|
| Sigmaelectrónica | ✅ Activa (219 productos) | Funcionando con selectores Kallyas | `stores/sigmaelectronica.go` |
| Electronilab | ⛔ Desactivada | Migró a Algolia InstantSearch (JS) — goquery no puede extraer productos | `stores/electronilab.go` |
| Vistronica | ⛔ Desactivada | Dominio `www.vistronica.com.co` no resuelve en DNS — tienda aparentemente inactiva | `stores/vistronica.go` |
| I+D Electrónica | 🔲 Pendiente | Fase 1 ampliada | — |
| JC Electrónica | 🔲 Pendiente | Fase 1 ampliada | — |
| Suconel | 🔲 Pendiente | Fase 2 | — |
| Ferretrónica | 🔲 Pendiente | Fase 2 | — |

> **Electronilab y Vistronica** tienen código de parser y fixtures en `testdata/` pero están marcadas `is_active = false` en la DB (migración `010` y `011` respectivamente). Reactivar cuando se resuelva el bloqueador.

## Extensión a nuevas tiendas

1. Agregar fila en `stores` + `scrape_rules` vía migración
2. Si WooCommerce estándar: los selectores de la tabla arriba funcionan directamente
3. Si estructura custom: crear `stores/<nombre>.go` con parser específico
4. Crear fixture HTML en `testdata/<nombre>/` + test unitario
5. Verificar con `go run ./cmd/scraper --store=<nombre> --dry-run`

## Desarrollo local

```bash
# Dry-run: parsea y muestra listings sin escribir a DB
go run ./cmd/scraper --dry-run

# Solo una tienda
go run ./cmd/scraper --store=sigmaelectronica --dry-run

# Tests unitarios (sin DB)
go test ./internal/scraping/...

# Tests de integración (requiere TEST_DATABASE_URL)
make test-integration
```
