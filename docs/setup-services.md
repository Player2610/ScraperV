# Setup de Servicios Externos — protou

Checklist de lo que hay que configurar en Neon, GCP y Cloudflare antes de poder hacer el primer deploy.

---

## Gotchas y fixes conocidos (2026-04-08)

| Problema | Fix |
|----------|-----|
| `Dockerfile` usaba `golang:1.22` pero `go.mod` requiere `go 1.23` | Cambiar imagen base a `golang:1.24-alpine` |
| `docker-compose.yml` usaba `ankane/pgvector:pg16` (deprecada) | Cambiar a `pgvector/pgvector:pg16` |
| `sslmode=req` en `DATABASE_URL` de Neon | `lib/pq` solo acepta `sslmode=require` (forma completa) |
| `CREATE EXTENSION pgvector` falla en Neon | El nombre real de la extensión es `vector`, no `pgvector` |
| `make migrate-up` no cargaba `.env` | Agregar `-include .env` + `export` al inicio del Makefile |
| WSL2 no resuelve dominios `.com.co` | Fijar DNS: `echo "nameserver 8.8.8.8" \| sudo tee /etc/resolv.conf`; hacer permanente en `/etc/wsl.conf` |
| Frontend no cargaba estilos Tailwind | Faltaba `web/tailwind.config.js` — sin él el plugin no sabe qué archivos escanear |
| Scraper dry-run devolvía 0 listings en Sigma | `dryFetchURL` usaba UA de bot; las tiendas sirven HTML vacío a bots |
| `GOOGLE_MAPS_API_KEY` vacío causaba panic al arrancar | Cambiado de `requireEnv()` a `os.Getenv()` — es opcional |
| Email remitente igual en dev y prod | Usar `NOTIFICATIONS_FROM` env var: `protou DEV <pedidos+dev@protou.co>` en dev, `protou <pedidos@protou.co>` en prod |

---

## 0. Entorno de desarrollo local

### Arquitectura dev vs prod

| | Dev | Prod |
|-|-----|------|
| **DB** | pgvector en Docker (volumen local) | Neon (PostgreSQL serverless) |
| **API** | Docker container (`:8080`) | GCP Cloud Run |
| **Frontend** | `bun dev` en host (`:4321`) | Cloudflare Pages |
| **Datos** | Sincronizados desde prod al arrancar | Fuente de verdad |

> Los datos solo fluyen **prod → dev**. Nunca al revés.

### Setup inicial

```bash
# Variables de entorno
cp .env.example .env           # credenciales de producción (Neon, Resend, JWT)
cp .env.dev.example .env.dev   # overrides dev — completar PROD_DATABASE_URL con el valor de DATABASE_URL de .env
cp web/.env.example web/.env   # PUBLIC_API_URL=http://localhost:8080

# Stack de backend (sincroniza DB desde Neon automáticamente)
make dev

# Frontend (en otra terminal)
cd web && bun install && bun dev
```

### Comandos Makefile

| Comando | Qué hace |
|---------|----------|
| `make dev` | Full start: `db → db-sync → migrate → api` |
| `make dev-quick` | Reinicia solo el API (útil cuando la DB ya está corriendo) |
| `make dev-reset` | Borra volumen + sincroniza desde cero |
| `make migrate-up` | Aplica migraciones pendientes (en host, hacia Neon si no hay `.env.dev`) |
| `make scraper-dry-run` | Parsea listings sin escribir a DB |

### Flujo de sincronización DB

Al hacer `make dev`, el servicio `db-sync` corre `pg_dump` contra Neon y restaura con `pg_restore --clean --if-exists` en el contenedor local. El API arranca con los datos exactos de producción.

---

## 1. Neon (Base de datos — Producción)

Neon es el PostgreSQL serverless que actúa como **base de producción**. En desarrollo se usa una DB local en Docker que se sincroniza desde Neon al arrancar (solo lectura desde dev — los datos nunca fluyen de vuelta).

### Requisitos
- Cuenta en [neon.tech](https://neon.tech) (free tier suficiente)

### Pasos
1. Crear un proyecto llamado `protou`
2. Crear una base de datos `neondb` (la que crea Neon por defecto sirve)
3. Verificar que la extensión `vector` esté disponible:
   ```sql
   SELECT * FROM pg_available_extensions WHERE name = 'vector';
   ```
4. Copiar el connection string y guardarlo en `.env` como `DATABASE_URL` (producción):
   ```
   DATABASE_URL=postgresql://user:pass@host/neondb?sslmode=require
   ```
5. Copiar el mismo valor en `.env.dev` como `PROD_DATABASE_URL` (fuente del sync):
   ```
   PROD_DATABASE_URL=postgresql://user:pass@host/neondb?sslmode=require
   ```
6. Verificar que `.env` y `.env.dev` están en `.gitignore` ✅

### Límites del free tier
| Recurso | Límite |
|---------|--------|
| Storage | 0.5 GB |
| Compute | 190 horas/mes (auto-suspend incluido) |
| Branches | 10 |
| pgvector | Incluido |

---

## 2. GCP (Backend — API + Scraper + Producción DB)

### 2.1 Proyecto GCP

1. Crear proyecto GCP con nombre `protou` (anota el `PROJECT_ID`)
2. Vincular cuenta de facturación (requerida para Cloud Run, aunque el free tier cubre el MVP)
3. Habilitar las APIs necesarias:
   ```bash
   gcloud services enable \
     run.googleapis.com \
     cloudscheduler.googleapis.com \
     secretmanager.googleapis.com \
     artifactregistry.googleapis.com \
     cloudbuild.googleapis.com \
     sqladmin.googleapis.com
   ```

### 2.2 Service Accounts

Crear tres service accounts con roles de menor privilegio:

```bash
export PROJECT_ID="<tu-project-id>"

# API — corre el Cloud Run Service
gcloud iam service-accounts create protou-api \
  --display-name "protou API"
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:protou-api@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/cloudsql.client"
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:protou-api@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

# Scraper — corre el Cloud Run Job
gcloud iam service-accounts create protou-scraper \
  --display-name "protou Scraper"
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:protou-scraper@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/cloudsql.client"
gcloud projects add-iam-policy-binding $PROJECT_ID \
  --member="serviceAccount:protou-scraper@$PROJECT_ID.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

# Deployer — usado por GitHub Actions CI/CD
gcloud iam service-accounts create protou-deployer \
  --display-name "protou CI/CD Deployer"
for ROLE in roles/run.admin roles/artifactregistry.writer roles/iam.serviceAccountUser; do
  gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:protou-deployer@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="$ROLE"
done
```

### 2.3 Artifact Registry

```bash
gcloud artifacts repositories create protou \
  --repository-format docker \
  --location us-central1 \
  --description "protou Docker images"

# Política de limpieza — mantiene las últimas 5 versiones (≈375 MB, dentro del free tier de 0.5 GB)
gcloud artifacts repositories set-cleanup-policies protou \
  --location us-central1 \
  --policy '[
    {"name":"keep-last-5","action":{"type":"Keep"},"mostRecentVersions":{"keepCount":5}},
    {"name":"delete-old","action":{"type":"Delete"},"condition":{"olderThan":"30d"}}
  ]'
```

### 2.4 Secret Manager

Crear los secrets que el API y el scraper consumen en runtime:

```bash
# Reemplazar los valores reales antes de ejecutar
echo -n "postgresql://..." | gcloud secrets create DATABASE_URL --data-file=-
echo -n "tu-jwt-secret-32-chars-min" | gcloud secrets create JWT_SECRET --data-file=-
echo -n "re_..." | gcloud secrets create RESEND_API_KEY --data-file=-
echo -n "AIza..." | gcloud secrets create GOOGLE_MAPS_API_KEY --data-file=-
```

### 2.5 Cloud Scheduler (trigger del scraper)

```bash
gcloud scheduler jobs create http protou-scraper-nightly \
  --location us-central1 \
  --schedule "0 7 * * *" \
  --uri "https://us-central1-run.googleapis.com/apis/run.googleapis.com/v1/namespaces/$PROJECT_ID/jobs/protou-scraper:run" \
  --message-body "" \
  --oauth-service-account-email protou-deployer@$PROJECT_ID.iam.gserviceaccount.com
```

> El cron `0 7 * * *` UTC equivale a las 02:00 COT (UTC-5).

### 2.6 Cloud SQL (producción solamente)

Solo necesario cuando se activa producción real (no para el MVP en desarrollo):

- Tier: `db-f1-micro` (~$9 USD/mes)
- Versión: PostgreSQL 16
- Región: `us-central1`
- Habilitar extensión `pgvector` después de crear la instancia:
  ```sql
  CREATE EXTENSION IF NOT EXISTS vector;
  ```

### 2.7 Workload Identity Federation (GitHub Actions sin JSON keys)

Ver [`gcp-cicd-setup.md`](./gcp-cicd-setup.md) para el setup paso a paso completo.

Secrets que hay que agregar en GitHub → Settings → Secrets → Actions:

| Secret | Valor |
|--------|-------|
| `GCP_PROJECT_ID` | Tu project ID |
| `GCP_REGION` | `us-central1` |
| `WIF_PROVIDER` | `projects/.../providers/github-provider` |
| `WIF_SERVICE_ACCOUNT` | `protou-deployer@<PROJECT_ID>.iam.gserviceaccount.com` |
| `API_SERVICE_ACCOUNT` | `protou-api@<PROJECT_ID>.iam.gserviceaccount.com` |

### Free tier GCP — resumen

| Servicio | Free tier |
|----------|-----------|
| Cloud Run Service | 2M requests/mes, 360k GB-s |
| Cloud Run Jobs | 180k vCPU-s, 360k GB-s |
| Cloud Scheduler | 3 jobs |
| Secret Manager | 6 secrets activos, 10k accesos/mes |
| Artifact Registry | 0.5 GB |
| Cloud SQL | **No tiene free tier** — $9/mes aprox. |

---

## 3. Cloudflare (Frontend — Astro SSR)

### Requisitos
- Cuenta en [cloudflare.com](https://cloudflare.com) (free plan suficiente)
- Repo de GitHub conectado a Cloudflare Pages

### Pasos

1. Ir a **Cloudflare Dashboard → Workers & Pages → Create → Pages → Connect to Git**
2. Seleccionar el repo `protou`
3. Configurar el build:

   | Campo | Valor |
   |-------|-------|
   | Framework preset | Astro |
   | Build command | `cd web && bun run build` |
   | Build output directory | `web/dist` |
   | Root directory | `/` (raíz del repo) |

4. Agregar las variables de entorno en Pages → Settings → Environment variables:

   | Variable | Valor |
   |----------|-------|
   | `PUBLIC_API_URL` | `https://api.protou.co` (o la URL de Cloud Run) |

5. Activar **Preview deployments** para que cada PR genere un ambiente de preview automático.

6. (Opcional) Dominio custom: Pages → Custom domains → `protou.co` o `www.protou.co`

### Comportamiento por rama

| Rama | Ambiente | URL |
|------|----------|-----|
| `main` | Producción | `protou.pages.dev` / dominio custom |
| Cualquier PR | Preview | `<branch>.protou.pages.dev` |

### Free tier Cloudflare Pages

| Recurso | Límite |
|---------|--------|
| Builds/mes | 500 |
| Requests | Ilimitados |
| Bandwidth | Ilimitado |
| Dominios custom | Ilimitados |
| SSR (Workers) | 100k requests/día |

> El SSR de Astro corre sobre Cloudflare Workers. El free tier de Workers tiene límite de 100k requests/día — más que suficiente para el MVP.

---

## Checklist de activación

### Desarrollo local
- [x] Cuenta Neon creada, proyecto `protou`, `DATABASE_URL` en `.env`
- [x] Migraciones aplicadas en Neon (versión 13)
- [x] Scraper corriendo — 219 listings de Sigmaelectrónica ingresados
- [x] `.env.dev` configurado con `PROD_DATABASE_URL` apuntando a Neon
- [x] `make dev` funciona: `db → db-sync → migrate → api` en Docker
- [x] `make dev-quick` y `make dev-reset` disponibles
- [x] `cd web && bun install && bun dev` corre sin errores (`:4321`)

### Staging / CI
- [ ] Proyecto GCP creado, APIs habilitadas
- [ ] Service accounts `protou-api`, `protou-scraper`, `protou-deployer` creados con roles
- [ ] Artifact Registry `protou` creado con política de limpieza
- [ ] Workload Identity Federation configurado (ver `gcp-cicd-setup.md`)
- [ ] Secrets de GitHub Actions configurados (5 secrets)
- [ ] Neon branch `staging` o base `protou_staging` disponible

### Producción
- [ ] Secrets en Secret Manager (`DATABASE_URL`, `JWT_SECRET`, `RESEND_API_KEY`, `GOOGLE_MAPS_API_KEY`)
- [ ] Cloud Scheduler job `protou-scraper-nightly` creado
- [ ] Cloudflare Pages conectado al repo con build command correcto
- [ ] Variable `PUBLIC_API_URL` en Cloudflare Pages apuntando al Cloud Run URL
- [ ] (Cuando corresponda) Cloud SQL instancia creada con pgvector habilitado
