# Infraestructura

## Diagrama

```
Cloud Scheduler ──── trigger nightly 02:00 COT ────► Cloud Run Job (scraper)
                                                              │
                                                              ▼
Student Browser ──► Cloudflare Pages (Astro) ──► Cloud Run Service (api) ◄──► Cloud SQL / Neon
                                                              │
Operator Browser ──────────────────────────────────────────┘
                                                              │
                                                       Secret Manager
                                                  (DB URL, JWT secret,
                                                   Resend key, Maps key)
```

---

## Servicios GCP

| Servicio | Uso | Free tier |
|----------|-----|-----------|
| Cloud Run Service | `cmd/api` — HTTP server | 2M requests/mes |
| Cloud Run Job | `cmd/scraper` — job nocturno | 180k vCPU-sec/mes |
| Cloud Scheduler | Trigger cron del scraper | 3 jobs gratis |
| Secret Manager | Credenciales y secrets | 6 secrets activos gratis |
| Artifact Registry | Imágenes Docker (`api`, `scraper`, `migrate`) | 0.5 GB gratis (~375 MB con política keepCount=5) |

---

## Base de datos

### Estrategia $0 antes de monetizar

Cloud SQL **no tiene free tier**. Solución:

| Entorno | Proveedor | Costo |
|---------|-----------|-------|
| Dev / Staging | [Neon](https://neon.tech) (serverless PostgreSQL) | Gratis (0.5 GB, pgvector incluido) |
| Producción | Cloud SQL `db-f1-micro` | ~$9 USD/mes |

Ambos son PostgreSQL estándar — las migraciones (`goose`) corren en los dos sin cambios.

### Por qué PostgreSQL

- Portable: se puede migrar a cualquier proveedor (Neon, Supabase, Railway, Cloud SQL, RDS)
- JSONB para snapshots de dirección y rutas de delivery
- `tsvector` / `tsquery` para búsqueda full-text sin Elasticsearch
- `pgvector` para embeddings del chatbot futuro (Fase 2)

---

## Frontend

**Cloudflare Pages** — gratis, CDN global, preview deployments automáticos por PR.

- Build command: `cd web && bun run build`
- Output: `web/dist`
- `PUBLIC_API_URL` apunta a `https://api.protou.co/v1`
- SSR manejado por el adapter `@astrojs/cloudflare`

---

## Containers

```
Dockerfile (multi-stage, multi-target)
  ├── --target api      → Cloud Run Service
  ├── --target scraper  → Cloud Run Job
  └── --target migrate  → Cloud Run Job (ejecutado en cada deploy)
```

Base final: `gcr.io/distroless/static` o `alpine`. Imágenes < 50 MB.

Todos los secrets se inyectan desde Secret Manager — sin variables de entorno planas en producción.

---

## CI/CD

### GitHub Actions

Tres workflows independientes:

| Archivo | Trigger | Qué hace |
|---------|---------|----------|
| `ci.yml` | Push + PR | lint → test (+ integración) → build-check (solo PR) |
| `deploy-api.yml` | Push a `main` | Build+push `api:SHA` + `migrate:SHA` → migraciones → deploy Cloud Run Service → smoke test `/health` |
| `deploy-scraper.yml` | Push a `main` | Build+push `scraper:SHA` → actualiza Cloud Run Job |

`deploy-api` y `deploy-scraper` corren **en paralelo** con CI al hacer push a `main`. El `build-check` (Docker sin push) corre solo en PRs.

### Autenticación GCP sin service account keys

Se usa **Workload Identity Federation (WIF)**: GitHub Actions obtiene un token OIDC y GCP lo verifica directamente sin JSON keys almacenadas en GitHub.

Secrets requeridos en GitHub → Settings → Secrets → Actions:

| Secret | Valor |
|--------|-------|
| `GCP_PROJECT_ID` | ID del proyecto GCP |
| `GCP_REGION` | `us-central1` |
| `WIF_PROVIDER` | `projects/.../providers/github-provider` |
| `WIF_SERVICE_ACCOUNT` | `protou-deployer@<PROJECT_ID>.iam.gserviceaccount.com` |
| `API_SERVICE_ACCOUNT` | `protou-api@<PROJECT_ID>.iam.gserviceaccount.com` |

Ver [gcp-cicd-setup.md](./gcp-cicd-setup.md) para el setup paso a paso.

### Artifact Registry — limpieza automática

Cada push a `main` crea una imagen con tag SHA. Sin limpieza el storage acumula y supera el free tier (0.5 GB).

Política configurada en el repo `protou` de Artifact Registry:
- Conserva las **últimas 5 versiones** de cada imagen (`api`, `scraper`, `migrate`)
- Borra imágenes con más de 30 días (si ya hay >5 versiones)
- GCP la aplica automáticamente cada 24h — sin cron, sin workflow

Cálculo: 3 imágenes × 5 versiones × ~25 MB = **~375 MB** → dentro del free tier de 0.5 GB.

### Cloudflare Pages

Deploy automático en push a `main`. Preview automático en PRs.

---

## Configuración local

`docker-compose.yml` levanta:
- `db`: postgres:16 con extensión pgvector (puerto 5432)
- `api`: target `api`, depende de `db`

Variables requeridas (documentadas en `.env.example`):

```
DATABASE_URL=postgres://postgres:password@localhost:5432/protou?sslmode=disable
PROD_DATABASE_URL=postgresql://<user>:<pass>@<host>/<db>?sslmode=require
JWT_SECRET=
RESEND_API_KEY=
GOOGLE_MAPS_API_KEY=
PORT=8080
CORS_ORIGIN=http://localhost:4321
NOTIFICATIONS_FROM=protou DEV <pedidos+dev@protou.co>
ALERT_EMAIL=
```

---

## Rollback

| Componente | Procedimiento |
|-----------|---------------|
| API | `gcloud run services update-traffic protou-api --to-revisions=PREV=100` |
| Scraper | Rollback de imagen en Cloud Run Job (revision anterior) |
| DB migration | `goose down` + deploy del binario anterior. Todas las migrations son aditivas (sin `DROP`) durante el MVP |
| Tarifa de delivery | Editar `delivery_fee_brackets` desde el panel — sin redeploy |
| ScrapeRule rota | Editar selector desde el panel — sin redeploy |

---

## Monitoreo (Fase 5)

Alert policies en Cloud Monitoring:
1. API error rate > 5% por 5 min → email
2. Cloud Run Job falla (exit code != 0) → email
3. Cloud SQL CPU > 80% por 10 min → email

Dashboard: request count, error rate, p99 latency, scraper success/failure rate.

---

## Cloud Monitoring Alerts

### Error Rate Alert

- **Condición**: Respuestas HTTP 5xx > 1% de los requests en una ventana de 5 minutos.
- **Notificación**: email al equipo.
- **Pasos**:
  1. Cloud Console → **Monitoring** → **Alerting** → **Create Policy**.
  2. Métrica: `Cloud Run Revision > Request Count`.
  3. Filtros: `service_name = protou-api`, `response_code_class = 5xx`.
  4. Configurar umbral: > 1% del total de requests en ventana de 5 minutos (usar ratio con métrica total).
  5. Notificación: agregar canal de email del equipo.
  6. Nombre: `protou-api-high-error-rate`.

### Scraper Failure Alert

- **Condición**: Cloud Run Job `scraper` finaliza con exit code != 0 (resultado `failed`).
- **Notificación**: email al equipo.
- **Pasos**:
  1. Cloud Console → **Monitoring** → **Alerting** → **Create Policy**.
  2. Métrica: `Cloud Run Job > Completed Execution Count`.
  3. Filtro: `job_name = scraper`, `result = failed`.
  4. Umbral: count > 0 en los últimos 60 minutos.
  5. Notificación: agregar canal de email del equipo.
  6. Nombre: `protou-scraper-job-failure`.

### Scraper Yield Drop Alert

- **Condición**: El scraper devuelve < 20% del promedio histórico de listings encontrados.
- **Notificación**: email al equipo (indica que los selectores CSS/XPath pueden haberse roto).
- **Pasos**:
  1. Esta alerta se implementa en la lógica del scraper (anomaly detection en `internal/scraping/worker.go`) con notificación vía `notifier.SendScraperAlert`.
  2. Para una alerta nativa de Cloud Monitoring, crear una **Custom Metric** publicada desde el scraper con el conteo de listings encontrados por ejecución.
  3. Cloud Console → **Monitoring** → **Alerting** → **Create Policy** → Custom Metric `custom.googleapis.com/scraper/listings_found`.
  4. Umbral: valor < 80% del promedio de los últimos 7 días.
  5. Nombre: `protou-scraper-yield-drop`.

---

## Alertas Cloud Monitoring

Configurar las siguientes alertas en Google Cloud Monitoring para el proyecto GCP de protou.

### 1. High Error Rate (5xx)

**Métrica**: `run.googleapis.com/request_count` con filtro `response_code_class=5xx`
**Condición**: tasa de 5xx > 5% del total de requests durante 5 minutos consecutivos
**Pasos**:
1. Cloud Console → Monitoring → Alerting → Create Policy
2. Add Condition → Metric: Cloud Run Revision > Request Count
3. Filter: `response_code_class = "5xx"`, resource: el service `api`
4. Configurar rolling window de 5 min, threshold > 5% del total
5. Notification channel: Slack o email del equipo

### 2. Scraper Job Failure

**Métrica**: `run.googleapis.com/job/completed_execution_count` con filtro `result=failed`
**Condición**: cualquier ejecución del job `scraper` termina con resultado `failed`
**Pasos**:
1. Cloud Console → Monitoring → Alerting → Create Policy
2. Add Condition → Metric: Cloud Run Job > Completed Execution Count
3. Filter: `result = "failed"`, job_name = "scraper"
4. Threshold: > 0 en cualquier intervalo de 1 minuto
5. Notification channel: Slack o email del equipo

### 3. Scraper Yield Drop

**Métrica**: Custom metric o Log-based metric sobre el conteo de listings scrapeados
**Condición**: conteo de listings scrapeados < 20% del promedio histórico de los últimos 7 días
**Pasos**:
1. Crear log-based metric: Logging → Log-based Metrics → Create
   - Nombre: `scraper_listings_count`
   - Filtro: logs del job scraper que contengan el conteo de listings
   - Tipo: Distribution sobre el campo numérico del conteo
2. Cloud Console → Monitoring → Alerting → Create Policy
3. Add Condition → Metric: la log-based metric creada
4. Usar Forecast condition o comparar contra baseline de 7 días
5. Notification channel: Slack o email del equipo

> **Nota**: Para la alerta de yield drop se recomienda que el scraper emita un log estructurado con el campo `listings_scraped: <n>` al finalizar cada job, para facilitar la log-based metric.
