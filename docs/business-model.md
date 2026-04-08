# Modelo de negocio

## Estrategia de costos

**El objetivo es mantener el costo de infraestructura en $0 antes de monetizar.**

Todas las decisiones de infra deben evaluarse contra este principio. No se activa ningún servicio de pago hasta que el proyecto genere ingresos reales.

Esto implica:
- Usar tiers gratuitos de GCP mientras el tráfico lo permita (Cloud Run tiene free tier, Cloud SQL no — ver alternativas para dev/staging)
- Cloudflare como alternativa a GCP si los costos de Cloud Run/Cloud SQL superan el free tier en las etapas iniciales
- Sin WhatsApp Business API (tiene costo por mensaje) hasta post-monetización
- Sin pasarela de pagos online hasta que el volumen lo justifique
- Servicios de terceros (geocodificación, email) elegidos por tener free tier suficiente para el MVP

Esta restricción se revisa una vez que el proyecto genere ingresos consistentes.

---

## Propuesta de valor

Estudiantes universitarios en Bogotá necesitan componentes electrónicos para sus proyectos pero tienen que buscar en múltiples tiendas dispersas, comparar precios manualmente y desplazarse. protou agrega los catálogos, muestra el mejor precio disponible y entrega en la dirección que el estudiante elija.

## Flujo de dinero

```
Estudiante paga (contra entrega)
  → Operador compra en tienda (precio scrapeado + buffer)
  → Operador cobra delivery
  → Margen = (precio vendido − precio tienda) + tarifa delivery
```

## Revenue streams

| Fuente | Mecanismo |
|--------|-----------|
| Markup | Precio de venta > precio scrapeado (buffer 5–10% absorbe fluctuaciones menores) |
| Delivery | Tarifa por distancia cobrada al estudiante |
| Ads | Anuncios no invasivos en el catálogo (Fase futura) |

## Pago

Contra entrega únicamente en MVP:
- Nequi
- Daviplata
- Llaves Breve
- Efectivo

Sin pasarela de pagos online. El operador registra el pago recibido en el panel después de entregar.

## Sin acuerdos comerciales (MVP)

La plataforma opera sobre scraping de catálogos públicos. No hay acuerdos previos con tiendas. Si una tienda solicita ser removida, se retira sin discusión. El plan a largo plazo es negociar acuerdos de datos/afiliado con las tiendas más frecuentes.

## Cobertura

Bogotá (todas las localidades) + Soacha. Validado en checkout. Fuera de zona → error claro al usuario.

## Tiendas objetivo (MVP)

- Sigmaelectrónica
- Electronilab
- Vistronica
- (agregar según disponibilidad de catálogo online)

## Restricciones de producto

Solo componentes de circuito: resistencias, capacitores, sensores, microcontroladores (Arduino, ESP32, etc.), módulos, cables dupont, protoboards, etc.

No vendemos dispositivos terminados (cargadores, audífonos, cámaras, etc.).
