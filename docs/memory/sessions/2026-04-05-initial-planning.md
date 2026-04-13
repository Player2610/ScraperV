# Session: Initial Planning

> Engram #100 | 2026-04-05

## Goal
Planificación completa inicial del proyecto protou: e-commerce dropshipping de componentes electrónicos para estudiantes universitarios en Bogotá.

## Instructions
- Usuario quiere CLAUDE.md lean: sin modelos de datos, sin estructura de archivos, solo lo que no se puede derivar del código o docs.
- Stack: Go + Astro + PostgreSQL/Cloud SQL + GCP Cloud Run.
- Equipo ~3 personas, desarrollo principalmente individual.
- Sin deadline, MVP debe ser sólido antes de salir.

## Discoveries
- Negocio: intermediario (compra física en tienda, entrega al estudiante). Pago contra entrega únicamente en MVP.
- La confirmación manual del operador es el control clave contra precios/stock desactualizados — nunca automatizar.
- pgvector debe activarse desde el inicio (habilitarlo en producción luego es costoso).
- Tarifa de delivery: por distancia con brackets en DB editables desde el panel. Multi-tienda: costo interno + descuento del 30% sobre el recargo, el usuario ve solo la tarifa final.
- Reglas de scraping en DB (no hardcodeadas) para poder corregir selectores sin redeploy.
- Cobertura: Bogotá + Soacha. Validación en checkout.
- Carrito persistente en DB (recuperable tras cerrar browser).
- Notificaciones: solo email en MVP. WhatsApp Business requiere aprobación Meta + costo.
- Fase 2 (no MVP): chatbot LLM + agrupación canónica de productos. El modelo de datos ya lo soporta (tags, CompatibilityNote, pgvector, category tree).

## Accomplished
- ✅ SDD inicializado en engram (topic: sdd-init/protou)
- ✅ Exploración completa guardada en engram (topic: sdd/initial-architecture/explore)
- ✅ Proposal completo guardado en engram (topic: sdd/initial-architecture/proposal)
- ✅ Skill registry en .atl/skill-registry.md y engram
- ✅ CLAUDE.md creado (lean, sin data models ni estructura de archivos)
- ✅ docs/ completa: business-model.md, phases.md, domains.md, data-model.md, scraping.md, delivery.md, operator-workflow.md, mvp-criteria.md

## Next Steps
- Continuar con sdd-design (modelo de datos detallado, contratos de API, límites de servicios)
- Luego sdd-spec (requisitos de comportamiento por dominio)
- Pueden correr en paralelo

## Relevant Files
- `/home/jufe/projects/protou/CLAUDE.md`
- `/home/jufe/projects/protou/docs/`
- `/home/jufe/projects/protou/.atl/skill-registry.md`
