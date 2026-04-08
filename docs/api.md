# API REST

**Base URL:** `https://api.protou.co/v1`

Todas las respuestas son JSON. Errores: `{"error": "mensaje", "code": "SNAKE_CASE_CODE"}`.

**Auth:**
- Estudiante: `Authorization: Bearer <jwt>`
- Operador: `Cookie: session=<token>`

**Headers de seguridad** incluidos en todas las respuestas: `X-Frame-Options`, `X-Content-Type-Options`, `Strict-Transport-Security`, `Content-Security-Policy-Report-Only`.

---

## Health Check (público)

```
GET /health
→ 200 { "status": "ok", "db": "ok" }
   | 503 { "status": "degraded", "db": "error", "error": "<mensaje>" }
```

Nota: este endpoint está en la raíz (`https://api.protou.co/health`), no bajo `/v1`.

---

## Catalog (público — sin auth)

```
GET  /listings
     ?q=<text>           búsqueda FTS
     &category_id=<id>   filtro por categoría (int)
     &store_id=<id>      filtro por tienda (int)
     &page=<n>           default 1
     &per_page=<n>       default 20, max 100
→ { listings: [...], total: N, page: N, per_page: N }

GET  /listings/{id}
→ { listing: Listing } | 404 si no existe o price_on_request

GET  /categories
→ { categories: [Category] }   árbol completo (children anidados)

GET  /categories/{slug}/listings?page=&per_page=
→ { category: Category, listings: [...], total: N, page: N, per_page: N }

GET  /stores
→ { stores: [{ id, name, base_url }] }
```

**Shape de Listing:**
```json
{
  "id": 42,
  "store": { "id": 1, "name": "Sigmaelectrónica", "base_url": "https://..." },
  "name": "Resistencia 10kΩ 1/4W",
  "description": null,
  "price_cop": 200,
  "stock_signal": "in_stock",
  "image_url": "https://...",
  "product_url": "https://...",
  "category": { "id": 5, "name": "Resistencias", "slug": "resistencias", "parent_id": 1, "icon_url": null },
  "last_scraped_at": "2026-04-06T02:13:00Z",
  "out_of_stock": false
}
```

`stock_signal` values: `in_stock`, `out_of_stock`, `unknown`, `price_on_request`.

---

## Auth — Estudiantes

```
POST /auth/register
Body: { email, full_name, password, phone?, habeas_data_consent? }
→ { user, token }   (201)
   | 400 MISSING_FIELDS       (email, full_name o password ausentes)
   | 400 INVALID_EMAIL        (formato de email inválido)
   | 400 PASSWORD_TOO_SHORT   (password < 8 caracteres)
   | 409 DUPLICATE_EMAIL

POST /auth/login
Body: { email, password }
→ { user, token } | 401 INVALID_CREDENTIALS
```

Auth endpoints tienen rate limiting: **10 requests/min por IP** (retorna 429 `RATE_LIMITED` al exceder).

Token is a signed HS256 JWT (24h expiry). Store in `localStorage` as `protou_token`.

---

## Users [RequireStudent]

```
GET    /users/me
→ { user: { id, email, phone, name, is_active, created_at, updated_at } }

DELETE /users/me
→ 202 { message: "Tu cuenta será eliminada en las próximas 24 horas" }
   Anonimiza los datos del usuario (derecho al olvido — Ley 1581/2012 HABEAS DATA).

GET    /users/me/addresses
→ { addresses: [Address] }

POST   /users/me/addresses
Body: { full_address, label?, reference?, lat?, lng? }
→ { address: Address } (201)

DELETE /users/me/addresses/{id}
→ 204 | 404 NOT_FOUND
```

**Shape de Address:**
```json
{
  "id": 3,
  "user_id": 7,
  "label": "Casa",
  "full_address": "Cra 7 #45-20, Bogotá",
  "reference": "Edificio azul, piso 3",
  "lat": 4.6486,
  "lng": -74.0788,
  "created_at": "2026-04-06T10:00:00Z"
}
```

---

## Cart [RequireStudent]

```
GET  /cart
→ { cart: { id, user_id, ... }, items: [CartItemEnriched] }

PUT  /cart/items/{listing_id}
Body: { quantity: N }   -- quantity: 1-99 (inclusive). Para eliminar un ítem usar DELETE abajo.
→ 204 | 400 INVALID_QUANTITY | 422 LISTING_UNAVAILABLE

DELETE /cart/items/{listing_id}
→ 204   -- elimina el ítem específico. Idempotente: 204 aunque el ítem no exista.

DELETE /cart
→ 204 (vacía todos los ítems)

POST /cart/migrate
Body: { items: [{ listing_id, quantity }] }   -- desde localStorage guest
→ { status: "ok" }   -- no retorna el carrito actualizado; usar GET /cart después si se necesita
```

**Shape de CartItemEnriched:**
```json
{
  "id": 5,
  "cart_id": 2,
  "listing_id": 42,
  "quantity": 3,
  "added_at": "...",
  "updated_at": "...",
  "listing": { ...Listing },
  "unavailable": false
}
```

---

## Checkout & Orders [RequireStudent]

```
POST /checkout/delivery-fee
Body: { address_id: N }
→ { total_fee: N, is_multi_store: bool, breakdown: "string" }
   | 422 OUTSIDE_ZONE

POST /orders
Body: {
  address_id: N,
  payment_method: "nequi"|"daviplata"|"efectivo"|"llaves_breve",
  notes?: string
}
→ { order: Order, items: [OrderItem] }   (201)
   | 400 MISSING_FIELDS           (payment_method ausente)
   | 400 INVALID_PAYMENT_METHOD   (valor no reconocido)
   | 422 OUTSIDE_ZONE | EMPTY_CART | UNAVAILABLE_ITEMS

GET  /orders
→ { orders: [Order] }

GET  /orders/{id}
→ { order: Order, items: [OrderItem] } | 403 FORBIDDEN | 404 NOT_FOUND
```

**Shape de Order:**
```json
{
  "id": 1,
  "user_id": 7,
  "status": "pending_confirmation",
  "delivery_address": { "full_address": "...", "label": null, "reference": null },
  "subtotal_cop": 12500,
  "delivery_fee_cop": 4000,
  "total_cop": 16500,
  "payment_method": "nequi",
  "notes": null,
  "created_at": "...",
  "updated_at": "..."
}
```

`status` values: `pending_confirmation`, `confirmed`, `purchasing`, `in_delivery`, `delivered`, `cancelled`, `failed`.

**Shape de OrderItem:**
```json
{
  "id": 1,
  "order_id": 1,
  "listing_id": 42,
  "listing_name_snapshot": "Resistencia 10kΩ 1/4W",
  "listing_store_snapshot": "Sigmaelectrónica",
  "price_snapshot_cop": 200,
  "quantity": 3,
  "is_cancelled": false,
  "created_at": "..."
}
```

---

## Operator Panel API

Auth: HttpOnly session cookie (`session=<token>`). Scope: `/v1/operator`. Expiry: 8h.
Login and logout are public; all other operator routes require `RequireOperatorSession` middleware.

### Auth

```
POST /v1/operator/auth/login
Body: { email: string, password: string }
→ 200 { operator_id: N } + Set-Cookie: session=<token>; HttpOnly; Secure; SameSite=Strict; Path=/v1/operator
   | 400 MISSING_FIELDS | 401 INVALID_CREDENTIALS | 429 RATE_LIMITED

POST /v1/operator/auth/logout
→ 204 + clears session cookie (deletes from operator_sessions table)
```

Rate limiting en operator login: **5 requests/min por IP**.

### Orders [RequireOperatorSession]

```
GET  /v1/operator/orders?status=<status>&page=<n>&per_page=<n>
     status filters by order status; omit for all
     default: page=1, per_page=20, max per_page=100
→ { orders: [OrderRow], page: N, per_page: N }

GET  /v1/operator/orders/{id}
→ { order: Order, items: [OrderItem], events: [OrderEvent], delivery: Delivery|null }

POST /v1/operator/orders/{id}/confirm
→ 200 { status: "confirmed" }
   Transitions pending_confirmation → confirmed (shortcut for the common case)

POST /v1/operator/orders/{id}/transition
Body: { to: string, note?: string }
     Allowed targets depend on current status — see state machine below
→ 200 { status: "<to>" } | 422 INVALID_TRANSITION | 404 NOT_FOUND

POST /v1/operator/orders/{id}/items/{item_id}/cancel
Body: { reason?: string }
→ 200 { new_subtotal_cop: N, new_total_cop: N, all_cancelled: bool }
   Recalculates order totals. If all items cancelled, order transitions to cancelled automatically.

PUT  /v1/operator/orders/{id}/delivery-fee
Body: { fee_cop: N }    (must be >= 0)
→ 200 { delivery_fee_cop: N, total_cop: N }
   Logs the change in order_events.

POST /v1/operator/orders/{id}/assign-courier
Body: { courier_id: N }
→ 200 { order_id: N, courier_id: N }
   Courier must be active. Order must be in purchasing or in_delivery status.
   Upserts deliveries row (reassignment allowed).

POST /v1/operator/orders/{id}/payment
Body: { method: string, amount_cop: N }
→ 201 { payment_id: N, order_id: N, method: string, amount_cop: N }
   Order must be in delivered status. One payment per order (409 DUPLICATE_PAYMENT if repeated).
```

**Shape of OrderRow** (returned by `GET /v1/operator/orders` list — includes student info):
```json
{
  "id": 1,
  "user_id": 7,
  "status": "pending_confirmation",
  "subtotal_cop": 12500,
  "delivery_fee_cop": 4000,
  "total_cop": 16500,
  "payment_method": "nequi",
  "notes": null,
  "created_at": "...",
  "updated_at": "...",
  "student_name": "Ana García",
  "student_email": "ana@unal.edu.co"
}
```

Note: `GET /v1/operator/orders/{id}` (detail) returns a richer `order` object that also includes `delivery_address` (JSON snapshot) and the same `student_name`/`student_email` fields.

**Shape of Delivery** (returned as `delivery` field in `GET /v1/operator/orders/{id}`; `null` if not yet assigned):
```json
{
  "id": 3,
  "order_id": 1,
  "courier_id": 2,
  "courier_name": "Carlos M.",
  "courier_phone": "3001234567",
  "assigned_at": "2026-04-06T10:00:00Z",
  "picked_up_at": null,
  "delivered_at": null,
  "delivery_fee_cop": 4000,
  "notes": null
}
```

**Order status state machine:**

```
pending_confirmation → confirmed, cancelled, failed
confirmed            → purchasing, cancelled, failed
purchasing           → in_delivery, cancelled, failed
in_delivery          → delivered, failed
delivered            → (terminal)
cancelled            → (terminal)
failed               → (terminal)
```

Transitions trigger async email notifications (fire-and-forget via Resend).

**Shape of OrderEvent:**
```json
{
  "id": 1,
  "order_id": 42,
  "from_status": "pending_confirmation",
  "to_status": "confirmed",
  "actor_id": 1,
  "note": "order confirmed by operator",
  "created_at": "2026-04-06T10:00:00Z"
}
```

### Couriers [RequireOperatorSession]

```
GET  /v1/operator/couriers
     ?all=true   include inactive couriers (default: active only)
→ { couriers: [Courier] }

POST /v1/operator/couriers
Body: { name: string, phone: string }
→ 201 { courier: Courier }

PUT  /v1/operator/couriers/{id}
Body: { name?: string, phone?: string, is_active?: bool }
     Partial patch — omitted fields unchanged
→ 200 { courier: Courier } | 404 NOT_FOUND
```

**Shape of Courier:**
```json
{ "id": 1, "name": "Carlos M.", "phone": "3001234567", "is_active": true, "created_at": "..." }
```

### Delivery Config [RequireOperatorSession]

```
GET /v1/operator/delivery-config
→ { brackets: [Bracket], multi_store_discount_pct: N, updated_at: "..." }

PUT /v1/operator/delivery-config
Body: {
  brackets: [{ distance_km_min: N, distance_km_max?: N, fee_cop: N }],
  multi_store_discount_pct: N
}
→ same shape as GET (returns updated config)
   Atomically replaces all brackets (DELETE + INSERT in transaction) + upserts config row.
   fee_cop must be > 0 per bracket.
```

### Logs [RequireOperatorSession]

```
GET /v1/operator/notification-logs
    ?order_id=<id>   filter by order (optional)
    Returns last 100 entries ordered by sent_at DESC
→ { notification_logs: [NotifLog] }

GET /v1/operator/scrape-jobs
    ?store_id=<id>   filter by store (optional)
    ?limit=<n>       default 20, max 100
→ { scrape_jobs: [ScrapeJob] }
```

**Shape of NotifLog:**
```json
{
  "id": 1,
  "order_id": 42,
  "user_id": 7,
  "channel": "email",
  "event": "order_confirmed",
  "sent_at": "2026-04-06T10:00:00Z",
  "status": "sent",
  "error_message": null
}
```

**Shape of ScrapeJob:**
```json
{
  "id": 5,
  "store_id": 1,
  "started_at": "2026-04-06T02:00:00Z",
  "finished_at": "2026-04-06T02:13:00Z",
  "status": "completed",
  "listings_found": 340,
  "listings_updated": 12,
  "listings_new": 3,
  "error_message": null
}
```

---

## Paginación

Todas las listas usan el mismo modelo:

```json
{
  "data": [...],
  "total": 250,
  "page": 2,
  "per_page": 20
}
```

## Códigos de error comunes

| HTTP | code | Situación |
|------|------|-----------|
| 400 | `BAD_REQUEST` / `MISSING_FIELDS` / `INVALID_*` | Request malformado o campo inválido |
| 401 | `UNAUTHORIZED` | Sin token o expirado |
| 403 | `FORBIDDEN` | Token válido pero sin permiso |
| 404 | `NOT_FOUND` | Recurso no existe o no visible |
| 409 | `CONFLICT` / `DUPLICATE_*` | Email duplicado, pago duplicado |
| 422 | `VALIDATION_ERROR` / `OUTSIDE_ZONE` / `EMPTY_CART` / `UNAVAILABLE_ITEMS` | Input inválido, regla de negocio violada |
| 429 | `RATE_LIMITED` | Demasiadas peticiones (auth endpoints) |
| 500 | `INTERNAL_ERROR` | Error inesperado |
