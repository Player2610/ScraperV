# Documentación — protou

## Negocio y operación

| Documento | Contenido |
|-----------|-----------|
| [business-model.md](./business-model.md) | Modelo de negocio, revenue streams, estrategia de costos $0 |
| [operator-workflow.md](./operator-workflow.md) | Ciclo de vida de una orden desde el panel del operador |
| [delivery.md](./delivery.md) | Tarifa por distancia, algoritmo multi-tienda, cobertura |
| [stores-discovery.md](./stores-discovery.md) | Mapa de tiendas scrapeadas (completar en Fase 1) |

## Arquitectura y diseño

| Documento | Contenido |
|-----------|-----------|
| [domains.md](./domains.md) | Bounded contexts, responsabilidades, interfaces públicas |
| [data-model.md](./data-model.md) | Schema SQL completo con índices y triggers |
| [api.md](./api.md) | Endpoints REST completos con shapes |
| [infrastructure.md](./infrastructure.md) | GCP, Neon, Cloudflare Pages, CI/CD, rollback |
| [gcp-cicd-setup.md](./gcp-cicd-setup.md) | Setup paso a paso: Workload Identity Federation + GitHub secrets |
| [tech-stack.md](./tech-stack.md) | Librerías, herramientas, convenciones de código |

## Implementación

| Documento | Contenido |
|-----------|-----------|
| [progress.md](./progress.md) | **Estado actual** — qué está hecho, qué está pendiente |
| [phases.md](./phases.md) | Fases 0–5 con scope y criterios de "done" |
| [mvp-criteria.md](./mvp-criteria.md) | Checklist de "listo para producción" |
| [scraping.md](./scraping.md) | Arquitectura del scraper, código, tiendas implementadas |
