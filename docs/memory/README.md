# docs/memory — Engram Memory Export

Exportación local de la memoria persistente de Engram para el proyecto **protou**.
Última actualización: 2026-04-13.

Esta carpeta es un respaldo legible del contexto de planificación y decisiones del proyecto.
La fuente de verdad es Engram (`project: protou`). Si hay discrepancia, Engram es canónico.

---

## Estructura

```
docs/memory/
├── project-context.md          # Stack, estado actual, decisiones clave
├── sdd/
│   ├── initial-architecture/   # Artefactos SDD — arquitectura inicial (Fases 0–4)
│   │   ├── explore.md
│   │   ├── proposal.md
│   │   ├── spec.md
│   │   ├── design.md
│   │   ├── tasks.md
│   │   ├── tasks-p2.md
│   │   └── apply-progress.md
│   └── hardening/              # Artefactos SDD — Fase 5 Hardening  ✅ exportado 2026-04-13
│       ├── explore.md
│       ├── proposal.md
│       ├── spec.md
│       ├── design.md
│       ├── tasks.md
│       └── apply-progress.md
├── sessions/                   # Resúmenes de sesiones de trabajo
│   ├── 2026-04-05-initial-planning.md
│   ├── 2026-04-05-sdd-complete.md
│   ├── 2026-04-07-pre-hardening-review.md
│   └── 2026-04-08-hardening-complete.md   ← sesión interrumpida, reconstruida
└── decisions/                  # Decisiones puntuales y descubrimientos
    ├── bun-package-manager.md
    ├── delete-cart-endpoint.md
    ├── phase0-foundation.md
    ├── test-review-pre-hardening.md
    ├── tests-hardening-review.md
    └── docs-hardening-review.md
```

## Índice de artefactos SDD

| Artefacto | Engram ID | Topic key |
|-----------|-----------|-----------|
| Exploración inicial | #98 | `sdd/initial-architecture/explore` |
| Proposal inicial | #99 | `sdd/initial-architecture/proposal` |
| Spec inicial | #102 | `sdd/initial-architecture/spec` |
| Design inicial | #101 | `sdd/initial-architecture/design` |
| Tasks inicial (Fases 0–2) | #103 | `sdd/initial-architecture/tasks` |
| Tasks inicial (Fases 3–5) | #104 | `sdd/initial-architecture/tasks-p2` |
| Apply progress inicial | #106 | `sdd/initial-architecture/apply-progress` |
| Exploración hardening | #111 | `sdd/hardening/explore` |
| Proposal hardening | #112 | `sdd/hardening/proposal` |
| Spec hardening | #113 | `sdd/hardening/spec` |
| Design hardening | #114 | `sdd/hardening/design` |
| Tasks hardening | #115 | `sdd/hardening/tasks` |
| Apply progress hardening | #116 | `sdd/hardening/apply-progress` |
