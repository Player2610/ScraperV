# Plan de despliegue — protou (repo ScraperV) — Plan B: sin GCP

## Contexto y cambio de rumbo

El plan original usaba GCP Cloud Run para API + scraper. **GCP quedó descartado**: habilitar
billing falla con `OR-CBAT-23` (rechazo de verificación de pago de Google). Cloud Run exige
billing incluso para el free tier, así que el backend se reubica a un proveedor con **free tier
sin tarjeta**.

## Arquitectura (todo $0, sin tarjeta)

| Componente | Servicio |
|-----------|----------|
| API (`cmd/api`) | **Render** Web Service (runtime Go nativo, plan free) |
| Migraciones (`cmd/migrate`) | **GitHub Actions** (`.github/workflows/migrate.yml`) + manual para el deploy inicial |
| Scraper (`cmd/scraper`) | **GitHub Actions** cron nocturno (`.github/workflows/scraper-cron.yml`) |
| Frontend (`web/`) | **Cloudflare Pages** |
| DB | **Neon** (PostgreSQL serverless + pgvector) |

> Render free: 750 h/mes, duerme tras ~15 min de inactividad (cold start ~30-60s). GitHub Actions:
> minutos gratis de sobra para un job nocturno.

## Archivos de despliegue en el repo

- `render.yaml` — Blueprint del API (runtime Go, health check `/health`, env vars).
- `.github/workflows/scraper-cron.yml` — scraper nocturno (cron `0 7 * * *` = 02:00 COT) + manual.
- `.github/workflows/migrate.yml` — migraciones en cambios de `db/migrations/**` + manual.
- `.github/workflows/ci.yml` — lint + test (Go 1.23, `pgvector/pgvector:pg16`).

## Pasos de despliegue

### Fase 0 — Cuentas (usuario, sin tarjeta)
1. **Neon** → proyecto `protou`, copiar connection string (`postgresql://...?sslmode=require`).
2. **Render** → cuenta (signup con GitHub).
3. **Cloudflare** → cuenta free.
4. **Resend** → API key `re_...`.

### Fase 1 — Migrar Neon
```
$env:DATABASE_URL="postgresql://...neon...?sslmode=require"; go run ./cmd/migrate
```
Crea schema + seeds (tiendas, categorías, delivery, operador `operator@protou.co`/`operator123`).

### Fase 2 — API en Render
1. Render → New → Blueprint → conectar repo → detecta `render.yaml`.
2. Completar env vars `sync:false`: `DATABASE_URL`, `JWT_SECRET` (`openssl rand -hex 32`),
   `RESEND_API_KEY`, `GOOGLE_MAPS_API_KEY` (opcional).
3. Render despliega → URL `https://protou-api.onrender.com`.
4. `curl <url>/health` → `{"status":"ok","db":"ok"}`.

### Fase 3 — Frontend en Cloudflare Pages
1. Connect to Git → repo `ScraperV`.
2. Build `cd web && bun run build` · Output `web/dist` · Root `/`.
3. Env `PUBLIC_API_URL` = URL de Render.
4. Anotar dominio `*.pages.dev` → setear `CORS_ORIGIN` en Render con ese valor → re-deploy.

### Fase 4 — Secrets de GitHub + scraper inicial
1. `gh secret set` en el repo: `DATABASE_URL`, `RESEND_API_KEY`, `ALERT_EMAIL` (opcional),
   `NOTIFICATIONS_FROM`.
2. Poblar listings: `gh workflow run scraper-cron.yml` o `go run ./cmd/scraper` local.
3. Verificar catálogo en el frontend y panel operador.

## Notas
- Cold start de Render free (~30-60s tras inactividad) — mitigable con ping cron o subiendo de plan.
- Migraciones aditivas del MVP → seguro desacoplarlas del deploy de Render.
- Dominio custom (`protou.co`): pasos de DNS extra más adelante.
