# Dominios del sistema

Cada dominio tiene su propio package en `internal/`. La dirección de dependencias es estricta — ver `docs/infrastructure.md`.

---

## Catalog (`internal/catalog`)

Dueño de toda la información sobre productos disponibles.

**Responsabilidades:**
- Mantener listings actualizados (precio, stock, imagen, URL)
- Organizar por categorías (árbol jerárquico, self-referential)
- Proveer búsqueda full-text vía PostgreSQL `tsvector`/`tsquery`
- Excluir listings no visibles del catálogo público

**Invariantes:**
- `(store_id, external_sku)` es único — ancla de deduplicación
- Listings con `price_cop = NULL` o `stock_signal = price_on_request` nunca visibles al estudiante
- Cambios en listings no afectan órdenes ya creadas (snapshots en `order_items`)

**Implementado:**
- `SearchListings(ctx, q string, filters ListingFilters, page Page) ([]Listing, int, error)`
- `GetListing(ctx, id int64) (Listing, error)`
- `ListCategoriesTree(ctx) ([]Category, error)`
- `GetCategoryBySlug(ctx, slug string) (Category, error)`
- `ListStores(ctx) ([]Store, error)`

Fase 2 (futura): introduce `Product` canónico que agrupa listings equivalentes entre tiendas.

---

## Scraping (`internal/scraping`)

Subsistema autónomo que alimenta Catalog. Corre como binario separado (`cmd/scraper`), nunca dentro del API.

**Responsabilidades:**
- Ejecutar jobs por tienda con reglas configuradas en DB
- Upsert de listings: crear si no existe, actualizar si cambió
- Registrar historial de precios en cada cambio
- Alertar al operador si un job falla o retorna resultados anómalos

**Invariantes:**
- ScrapeRules (selectores CSS/XPath) están en DB — editables sin redeploy
- Un job que retorna <20% del histórico de esa tienda dispara alerta inmediata
- Un job que retorna 0 listings siempre dispara alerta
- Precios en USD → `stock_signal = price_on_request` (sin conversión)

**Interfaz pública:**
```go
type Runner interface {
    RunAll(ctx) error
    RunStore(ctx, storeID int64) error
}
```

---

## Orders (`internal/orders`)

Ciclo de vida completo de una orden.

**Estados válidos:**
```
pending_confirmation → confirmed → purchasing → in_delivery → delivered
                    ↘                                        ↗
                     cancelled / failed  (desde cualquier estado pre-delivered)
```

**Responsabilidades:**
- Crear órdenes con snapshots inmutables en una sola transacción DB
- Registrar eventos de cambio de estado (trazabilidad completa)
- Permitir cancelación de ítems individuales sin cancelar la orden completa
- Exponer historial al estudiante y al operador

**Invariantes:**
- `OrderItem.price_snapshot_cop`, `listing_name_snapshot`, `listing_store_snapshot` son **inmutables** tras la creación
- `Order.delivery_address_snapshot` es JSONB inmutable
- Ninguna orden pasa a `confirmed` sin acción manual del operador — nunca auto-confirmar
- Cancelar todos los ítems → orden pasa a `cancelled` automáticamente

**Implementado (Fase 3):**
- `CreateOrder(ctx, userID int64, req CreateOrderRequest) (Order, []OrderItem, error)` — transacción atómica con snapshots
- `CalculateDeliveryFee(ctx, userID, addressID int64) (FeeResult, error)`
- `GetOrder(ctx, orderID, userID int64) (Order, []OrderItem, error)`
- `ListOrders(ctx, userID int64) ([]Order, error)`

Operaciones de gestión (Fase 4): `ConfirmOrder`, `TransitionStatus`, `CancelItem`, `OverrideDeliveryFee`.

---

## Delivery (`internal/delivery`)

Logística de última milla y cálculo de tarifas.

**Responsabilidades:**
- Calcular tarifa de delivery (ver [delivery.md](./delivery.md))
- Validar cobertura de zona (Bogotá/Soacha)
- Asignar mensajeros a órdenes
- Registrar timestamps de cada etapa

**Invariantes:**
- `DeliveryFeeCalculator.Calculate()` es una función pura — sin acceso a DB, sin efectos secundarios
- Las tarifas de brackets ya creados **no afectan órdenes existentes** — solo aplican a órdenes nuevas

**Implementado:**
```go
func Calculate(storeCoords []LatLng, deliveryCoord LatLng, brackets []FeeBracket, discountPct int) FeeResult
```

`FeeResult` incluye `TotalFee int`, `IsMultiStore bool`, `Breakdown string`.
El repositorio expone `GetBracketsAndConfig` y `GetCouriers`.

---

## Users (`internal/users`)

Cuentas de estudiantes y operadores.

**Responsabilidades:**
- Registro con consentimiento HABEAS DATA obligatorio
- Autenticación (JWT para estudiantes, sesión en DB para operadores)
- Gestión de direcciones guardadas por usuario
- Roles: `student`, `operator`, `admin`

**Implementado (Fase 3):**
- `Register(ctx, RegisterRequest) (User, token string, error)` — bcrypt hash, rol `student`
- `Login(ctx, email, password string) (User, token string, error)`
- `GetUserByID(ctx, id int64) (User, error)`
- `ListAddresses`, `AddAddress`, `DeleteAddress`

---

## Auth (`internal/auth`)

Capa transversal de autenticación y autorización. No tiene endpoints propios — provee middleware.

**Estudiantes:** JWT HS256, 24h de expiración, almacenado en `localStorage` del cliente.

**Operadores:** cookie `HttpOnly; Secure; SameSite=Strict`, sesión en tabla `operator_sessions` (8h). Limpieza de sesiones expiradas cada 1h vía goroutine en startup del API.

**Implementado:**
```go
func IssueToken(userID int64, role string) (string, error)
func ValidateToken(tokenStr string) (*Claims, error)
func RequireStudent(next http.Handler) http.Handler
func RequireOperator(next http.Handler) http.Handler
func StudentFromContext(ctx context.Context) (*Claims, bool)
func OperatorFromContext(ctx context.Context) (*Claims, bool)
func StartSessionCleanup(db *sql.DB)  // goroutine, cada 1h
```

---

## Cart (`internal/cart`)

Carrito de compras persistente en DB.

**Responsabilidades:**
- Un cart por usuario (creado al primer add)
- Cart guest en `localStorage` del browser — migrado a DB al hacer login
- Mostrar warning en ítems no disponibles desde que fueron agregados

**Invariantes:**
- El cart referencia listings **vivos** (sin snapshot) — el precio mostrado es el actual
- El snapshot solo ocurre al crear la orden en checkout
- Quantity 0 elimina el `CartItem` — no existe `CartItem` con `quantity = 0`
- Checkout rechazado si algún ítem es `out_of_stock`, `price_on_request`, o `is_active = false`

**Implementado:**
- `GetCart(ctx, userID int64) (CartResponse, error)` — enriquece ítems con listing data, marca `unavailable`
- `AddToCart(ctx, userID, listingID int64, quantity int) error`
- `MigrateGuestCart(ctx, userID int64, items []GuestItem) error`

---

## Payments (`internal/payments`)

Registro de pagos recibidos. MVP: contra entrega únicamente, sin pasarela de pagos.

**Responsabilidades:**
- Registrar método, monto, receptor y timestamp tras la entrega física
- No procesa ni mueve dinero — solo registra el hecho del pago

---

## Notifications (`internal/notifications`)

Comunicación con estudiante y operador sobre el estado de las órdenes.

**MVP:** email vía [Resend](https://resend.com) (3.000/mes gratis).

**Implementado (Fase 3):**
- `SendOrderCreated(ctx, to, userName string, order Order, items []OrderItem) error` — envía email HTML vía Resend, registra resultado en `notification_logs`. Nunca retorna error al caller (fallo logueado, nunca rollbackea la orden).
- Template HTML en `internal/notifications/templates/order_created.html` con fallback embebido.

**Triggers planificados:**

| Evento | Destinatario | Estado |
|--------|-------------|--------|
| Orden creada | Estudiante | ✅ implementado |
| Orden confirmada | Estudiante | Fase 4 |
| En camino | Estudiante | Fase 4 |
| Entregada | Estudiante | Fase 4 |
| Ítem(s) cancelados | Estudiante | Fase 4 |
| Orden cancelada | Estudiante | Fase 4 |

**Invariantes:**
- Fallo de notificación se loguea en `notification_logs` pero **nunca** rollbackea la orden
- Sin reintentos automáticos en MVP

Fase 2 (futura): WhatsApp Business API.
