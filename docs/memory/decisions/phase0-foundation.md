# Discovery: Phase 0 Foundation — All Tasks Implemented

> Engram #107 | 2026-04-06 | topic: `sdd/initial-architecture/phase0-complete`

**What**: All Phase 0 SDD tasks implemented for protou initial architecture. 40+ files created.

**Learned**:
- `go.sum` needs `go mod tidy` to be run (requires network): `cd /home/jufe/projects/protou && go mod tidy`
- The `db` package (`db/migrate.go`) uses `go:embed` — embed path `"migrations/*.sql"` is relative to `db/` directory
- `cmd/migrate` imports `github.com/protou/protou/db` to get the embedded FS
- Goose needs each DDL statement wrapped in `StatementBegin`/`StatementEnd`
- Migration 002 contains the COMPLETE schema — all tables, enums, indexes, FTS trigger, constraints
- Migration 003 seeds `delivery_config` (singleton) and 5 delivery fee brackets
- docker-compose uses `ankane/pgvector:pg16` image (has pgvector built-in)
- Dockerfile has 3 targets: `api`, `scraper`, `migrate` — all `alpine:3.19` final stage
- Astro frontend: SSR mode, `@astrojs/cloudflare` adapter, tailwindcss
- `web/src/lib/api.ts` has fully typed fetch wrappers for all endpoints
