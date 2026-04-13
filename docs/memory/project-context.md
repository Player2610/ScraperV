# Project Context — protou

> Engram #97 | topic: `sdd-init/protou` | Updated: 2026-04-11

## Stack

**Backend**: Go 1.23 (`github.com/protou/protou`)
- Router: `go-chi/chi v5`
- Auth: `golang-jwt/jwt v5` (student JWT) + cookie sessions (operator)
- DB driver: `lib/pq` (PostgreSQL)
- Migrations: `pressly/goose v3`
- Scraping: `PuerkitoBio/goquery`
- Rate limiting: `go-chi/httprate`
- Testing: `stretchr/testify` + `DATA-DOG/go-sqlmock`
- Commands: `cmd/api` (Cloud Run Service), `cmd/migrate`, `cmd/scraper` (Cloud Run Job)

**Frontend**: Astro 4 (`web/`)
- Package manager: bun@1.3.11
- Adapter: `@astrojs/cloudflare` (SSR)
- Styling: Tailwind CSS + `@astrojs/tailwind`
- TypeScript: yes

**Database**: PostgreSQL (Cloud SQL) with pgvector enabled from day 1

**Infrastructure**: GCP — Cloud Run, Cloud Scheduler, Cloud SQL. Cloudflare fallback.

## Internal Packages

`internal/`: auth, cart, catalog, delivery, e2e, notifications, operator, orders, payments, platform, scraping, users

## Current Phase

Phases 0–5 complete (including Fase 5 Hardening — 2026-04-07). System has:
- 82+ unit tests
- Full HTTP hardening (timeouts, graceful shutdown, security headers, rate limiting)
- JWT auth for students, cookie session for operators
- Cart, catalog, orders, delivery, operator panel
- Scraper with CSS/XPath rules in DB

## Architecture Non-Negotiables

1. No auto-confirm orders — operator reviews manually
2. Immutable snapshots in orders (price, name, address captured at creation)
3. pgvector from day 1 (for Phase 2 chatbot)
4. Scraping rules stored in DB (no redeploy to change selectors)
5. Cart persisted in DB
6. Phase 1: independent listings per store (no canonical grouping yet)

## SDD Artifacts (all in engram, project: protou)

- `sdd/initial-architecture/explore` — initial exploration
- `sdd/initial-architecture/proposal` — initial proposal
- `sdd/initial-architecture/spec` — behavioral specs
- `sdd/initial-architecture/design` — technical design
- `sdd/initial-architecture/tasks` — tasks Phase 0–2
- `sdd/initial-architecture/tasks-p2` — tasks Phase 3–5 (overflow)
- `sdd/initial-architecture/apply-progress` — Phase 4 complete
- `sdd/hardening/explore` — hardening exploration
- `sdd/hardening/proposal` — hardening proposal
- `sdd/hardening/spec` — hardening specs
- `sdd/hardening/design` — hardening design
- `sdd/hardening/tasks` — hardening tasks
- `sdd/hardening/apply-progress` — Phase 5 complete (2026-04-07)
