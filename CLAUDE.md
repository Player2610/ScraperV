# protou — Contexto del proyecto

## Qué es

Plataforma intermediaria (dropshipping) de componentes electrónicos para circuitos (resistencias, capacitores, sensores, microcontroladores, módulos) dirigida a estudiantes universitarios en Bogotá. El equipo compra físicamente en tiendas y entrega al estudiante. No vendemos dispositivos terminados.

## Modelo de negocio

- Ingresos: markup sobre precio de tienda + tarifa de delivery + anuncios no invasivos
- Pago: contra entrega (Nequi / Daviplata / Llaves Breve / efectivo). Sin pasarela de pagos por ahora.
- Sin acuerdos con tiendas — operamos sobre scraping de catálogos públicos.
- Cobertura: Bogotá + Soacha. Validado en checkout. Fuera de zona → orden rechazada.

## Stack

- Backend: Go (`cmd/api` Cloud Run Service, `cmd/scraper` Cloud Run Job)
- Frontend: Astro (SSR para páginas de producto, SSG para estáticas)
- DB: PostgreSQL en Cloud SQL con pgvector habilitado desde el inicio
- Infra: GCP (Cloud Run + Cloud Scheduler + Cloud SQL). Fallback: Cloudflare.
- Equipo: ~3 personas, desarrollo principalmente individual.

## Decisiones clave (no negociables)

1. **Sin confirmación automática de órdenes.** El operador revisa stock/precio manualmente antes de confirmar. Es la única barrera contra precios desactualizados.
2. **Snapshots inmutables en órdenes.** Precio, nombre del producto y dirección se capturan al crear la orden y nunca cambian.
3. **pgvector desde el día 1.** El chatbot (Fase 2) usará embeddings. Activarlo luego en producción es costoso.
4. **Reglas de scraping en DB.** Los selectores CSS/XPath se editan desde el panel sin redeploy.
5. **Carrito persistente en DB.** Recuperable si el usuario cierra el browser.
6. **Fase 1: listings independientes por tienda.** El usuario elige entre opciones del mismo componente. Agrupación automática es Fase 2.

## Contexto SDD

Los artefactos de planificación (proposal, specs, design, tasks) están en engram bajo el proyecto `protou`.
- Exploración: `sdd/initial-architecture/explore`
- Proposal: `sdd/initial-architecture/proposal`

Documentación detallada en `/docs/`.

## Fase 2 (no MVP)

- Chatbot LLM que recomienda productos según el proyecto del estudiante.
- Agrupación canónica de productos entre tiendas.
- El modelo de datos ya está diseñado para soportarlo (tags, categorías jerárquicas, CompatibilityNote, pgvector).
