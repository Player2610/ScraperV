# Modelo de delivery

## Tarifa por distancia

La tarifa base se calcula según la distancia haversine entre la dirección del estudiante y las tiendas involucradas. Los brackets son editables desde el panel del operador sin redeploy (tabla `delivery_fee_brackets`).

| Distancia (km) | Tarifa base (COP) |
|----------------|-------------------|
| 0 – 3          | 5.000             |
| 3 – 8          | 8.000             |
| 8 – 15         | 12.000            |
| 15 – 25        | 16.000            |
| 25+            | 20.000            |

Valores iniciales — ajustables desde el panel sin redeploy.

---

## Algoritmo de cálculo

La función `delivery.Calculate()` es **pura** — sin acceso a DB, sin efectos secundarios. El caller (orders service) carga los inputs desde DB y la llama.

### Una tienda

```
distance = haversine(store.lat/lng, delivery.lat/lng)
base_fee = bracket_lookup(distance, brackets)
total    = round_to_500(base_fee)
```

### Dos o más tiendas

```
centroid          = avg(store.lat), avg(store.lng)
distance          = haversine(centroid, delivery.lat/lng)
base_fee          = bracket_lookup(distance, brackets)
with_surcharge    = base_fee × 1.10              -- recargo fijo del 10% por múltiples recogidas
final_fee         = with_surcharge × (1 - multi_store_discount_pct / 100)
total             = round_to_500(final_fee)
```

El recargo del 10% compensa el esfuerzo adicional de recoger en varias tiendas. El descuento del operador se aplica sobre el monto ya recargado (default 30%).

### Redondeo

El total siempre se redondea al múltiplo de 500 COP más cercano.

### Ejemplo

```
Tienda A (Sigmaelectrónica) y Tienda B (Electronilab)
Distancia centroide → estudiante: 5 km → tarifa base: COP 8.000
Recargo 10%: COP 8.000 × 1,10 = COP 8.800
Descuento 30%: COP 8.800 × 0,70 = COP 6.160
Total antes de redondeo: COP 6.160 → redondeado: COP 6.000
```

---

## Lo que ve el usuario en checkout

El detalle interno nunca se expone. El estudiante ve:

```
Subtotal:                    COP 45.000
Delivery (multi-tienda):     COP 10.000  ← descuento ya incluido
─────────────────────────────────────────
Total:                       COP 55.000
```

El texto "multi-tienda" indica que hay descuento aplicado, sin revelar el cálculo.

---

## Cobertura

- Bogotá (todas las localidades)
- Soacha

### Validación de zona (MVP)

```go
func IsAddressCovered(fullAddress string) bool {
    covered := []string{"bogotá", "bogota", "soacha"}
    lower := strings.ToLower(fullAddress)
    for _, zone := range covered {
        if strings.Contains(lower, zone) {
            return true
        }
    }
    return false
}
```

Si la dirección no contiene ninguno de los términos → checkout rechazado con HTTP 422 y mensaje claro.

**Fase 2**: reemplazar por geocodificación + validación por polígono (Google Maps).

---

## Geocodificación

- **Coordenadas de tiendas**: fijas en DB, cargadas una vez al hacer seed (`004_seed_stores.sql`).
- **Coordenadas del estudiante**: geocodificadas al guardar la dirección en `Address.lat/lng` vía Google Maps Geocoding API (1 llamada, resultado persistido).
- **En checkout**: sin llamadas a Maps — se usan `lat/lng` ya almacenados. Si `lat = NULL` (geocodificación falló), se intenta de nuevo; si falla de nuevo → HTTP 422.

---

## Override del operador

Antes de confirmar una orden, el operador puede editar la tarifa de delivery desde el panel. El cambio queda registrado en `OrderEvent` con el valor anterior y el nuevo. Solo disponible antes de `confirmed`.

---

## Mensajeros

Los mensajeros se registran en `Courier`. La asignación es manual — el operador selecciona al mensajero disponible desde el panel al transicionar a `in_delivery`. Sin optimización de rutas en MVP.
