# Flujo del operador

El panel del operador está diseñado para ser usado desde un celular. El operador estará en la calle, no frente a un computador.

**Estado:** implementado en Fase 4. Acceso: `POST /v1/operator/auth/login` con email+password, sesión HttpOnly 8h.
Seed de desarrollo: `operator@protou.co` / `operator123` (migration 008).

## Ciclo de vida de una orden

```
pending_confirmation
  → confirmed
    → purchasing
      → in_delivery
        → delivered
```

Cancelación o fallo posible desde cualquier estado previo a `delivered`.

---

## 1. Nueva orden llega (`pending_confirmation`)

**Notificación:** email al estudiante (enviado al crear la orden).

**El operador debe:**
1. Abrir la orden en el panel (`GET /v1/operator/orders/{id}`)
2. Verificar cada ítem:
   - ¿El precio en la tienda coincide con el precio de la orden? (puede haber diferencia por scrape desactualizado)
   - ¿El producto está en stock?
3. Si todo está bien → **Confirmar** (`POST /v1/operator/orders/{id}/confirm`) → estado pasa a `confirmed`, email automático al estudiante
4. Si hay diferencia de precio menor (dentro del buffer de markup) → confirmar y absorber la diferencia
5. Si un ítem no está disponible → cancelar ese ítem (`POST /v1/operator/orders/{id}/items/{item_id}/cancel`) + email automático al estudiante. El total se recalcula automáticamente.
6. Si se cancelan todos los ítems, la orden queda cancelada automáticamente
7. Si la diferencia de precio es grande → contactar al estudiante antes de confirmar

**Regla crítica:** nunca confirmar automáticamente. Este paso es el único control contra precios o stock desactualizados.

---

## 2. Ir a comprar (`confirmed → purchasing`)

El operador marca la orden como `purchasing` al salir hacia la(s) tienda(s) (`POST /v1/operator/orders/{id}/transition` con `{ "to": "purchasing" }`).

Si la orden tiene productos de múltiples tiendas, el panel muestra la lista agrupada por tienda para minimizar desplazamientos.

---

## 3. En camino (`purchasing → in_delivery`)

El operador (o mensajero asignado) recoge los productos y se dirige a la dirección del estudiante.

El operador puede asignar un mensajero desde el panel (`POST /v1/operator/orders/{id}/assign-courier`) si no es él mismo quien entrega. El mensajero debe existir y estar activo en la tabla `couriers`.

Transición: `POST /v1/operator/orders/{id}/transition` con `{ "to": "in_delivery" }`.

El estudiante recibe email: "Tu pedido está en camino".

---

## 4. Entregado (`in_delivery → delivered`)

Al entregar, el mensajero (o el operador) confirma la entrega en el panel:
1. Transición a `delivered` (`POST /v1/operator/orders/{id}/transition` con `{ "to": "delivered" }`) — dispara email al estudiante
2. Registrar pago recibido (`POST /v1/operator/orders/{id}/payment`):
   - `method`: `nequi` | `daviplata` | `efectivo` | `llaves_breve`
   - `amount_cop`: monto recibido
   - Solo se puede registrar cuando el estado es `delivered`. Un pago por orden (409 si se repite).

El sistema envía email de confirmación al estudiante tras la transición a `delivered`.

---

## Cancelación parcial

El operador puede cancelar ítems individuales sin cancelar la orden completa:
- Ítem cancelado → se descuenta del total
- Si se cancelan todos los ítems → la orden queda cancelada automáticamente
- Email automático al estudiante con los ítems cancelados y razón

---

## Override de tarifa de delivery

Antes de confirmar, el operador puede ajustar la tarifa de delivery calculada si la situación real difiere del cálculo automático. El ajuste queda en el log de eventos.

---

## Vista del panel (diseño funcional)

```
ÓRDENES
━━━━━━━━━━━━━━━━━━━
⚡ Por confirmar (3)
  #1042 · Juan García · COP 57.000
  #1041 · María López · COP 23.500
  #1040 · Carlos Ruiz · COP 89.000

✓ Confirmadas (1)
  #1039 · Andrés M. · COP 45.000

📦 En compra (2)
  ...

🛵 En camino (1)
  ...
━━━━━━━━━━━━━━━━━━━
```

Cada orden muestra: número, nombre del estudiante, total, dirección resumida, y los ítems agrupados por tienda.
