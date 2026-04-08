# Setup CI/CD con GCP y GitHub Actions

Autenticación sin service account keys (Workload Identity Federation).
Ejecutar estos comandos **una sola vez** después de tener el proyecto GCP creado.

Reemplazar a lo largo de todo el documento:
- `<PROJECT_ID>` → tu GCP project ID (ej: `protou-abc123`)
- `<GITHUB_ORG>` → tu usuario u organización de GitHub (ej: `jufe`)
- `<REPO_NAME>` → nombre del repo (ej: `protou`)

---

## 0. Crear Artifact Registry con cleanup automático

Crear el repositorio **con política de limpieza incluida** — retiene solo las últimas 10 imágenes por imagen y borra el resto automáticamente. Así el storage nunca supera el free tier de 0.5 GB.

```bash
export PROJECT_ID="<PROJECT_ID>"
export REGION="us-central1"

# Crear el repositorio
gcloud artifacts repositories create protou \
  --project $PROJECT_ID \
  --repository-format docker \
  --location $REGION \
  --description "protou Docker images"

# Política de limpieza: conservar las 10 versiones más recientes de cada imagen
# Las más viejas se borran automáticamente — sin intervención manual
gcloud artifacts repositories set-cleanup-policies protou \
  --project $PROJECT_ID \
  --location $REGION \
  --policy '[
    {
      "name": "keep-last-10",
      "action": {"type": "Keep"},
      "mostRecentVersions": {"keepCount": 10}
    },
    {
      "name": "delete-old",
      "action": {"type": "Delete"},
      "condition": {"olderThan": "30d"}
    }
  ]'
```

**Qué hace la política:**
- Siempre conserva las 10 versiones más recientes de cada imagen (`api`, `scraper`, `migrate`)
- Borra cualquier imagen con más de 30 días **si ya hay más de 10 versiones** — nunca borra las últimas 10
- GCP la aplica automáticamente cada 24h — no hay cron, no hay workflow, no hay acción manual

**Cuánto ocupa en la práctica:**
- Imagen Go en Alpine ≈ 20–30 MB
- 3 imágenes × 10 versiones × 25 MB ≈ **750 MB máximo**
- Free tier: 0.5 GB. Con 10 versiones se supera ligeramente (~$0.02/mes extra por el exceso)
- Si querés mantenerte en 0: cambiar `keepCount: 5` → 3 imágenes × 5 × 25 MB ≈ 375 MB ✅

---

## 1. Variables de entorno para los comandos

```bash
export PROJECT_ID="<PROJECT_ID>"
export REGION="us-central1"
export GITHUB_ORG="<GITHUB_ORG>"
export REPO_NAME="<REPO_NAME>"
```

---

## 2. Crear service account deployer

Un único SA para CI/CD — no el mismo que corre el API o el scraper en producción.

```bash
gcloud iam service-accounts create protou-deployer \
  --project $PROJECT_ID \
  --display-name "protou CI/CD Deployer"
```

Darle los roles necesarios para hacer build, push y deploy:

```bash
for ROLE in \
  roles/run.admin \
  roles/artifactregistry.writer \
  roles/iam.serviceAccountUser; do
  gcloud projects add-iam-policy-binding $PROJECT_ID \
    --member="serviceAccount:protou-deployer@$PROJECT_ID.iam.gserviceaccount.com" \
    --role="$ROLE"
done
```

> `roles/iam.serviceAccountUser` es necesario para que el deployer pueda hacer deploy
> de Cloud Run Services que corren como `protou-api@...` y `protou-scraper@...`.

---

## 3. Crear Workload Identity Pool y Provider

```bash
# Crear el pool
gcloud iam workload-identity-pools create github-pool \
  --project $PROJECT_ID \
  --location global \
  --display-name "GitHub Actions Pool"

# Crear el provider (OIDC de GitHub)
gcloud iam workload-identity-pools providers create-oidc github-provider \
  --project $PROJECT_ID \
  --location global \
  --workload-identity-pool github-pool \
  --display-name "GitHub Actions Provider" \
  --attribute-mapping "google.subject=assertion.sub,attribute.repository=assertion.repository,attribute.actor=assertion.actor" \
  --attribute-condition "assertion.repository == '${GITHUB_ORG}/${REPO_NAME}'" \
  --issuer-uri "https://token.actions.githubusercontent.com"
```

Obtener el resource name del provider (lo necesitas para el secret de GitHub):

```bash
gcloud iam workload-identity-pools providers describe github-provider \
  --project $PROJECT_ID \
  --location global \
  --workload-identity-pool github-pool \
  --format "value(name)"
# Output: projects/123456/locations/global/workloadIdentityPools/github-pool/providers/github-provider
```

---

## 4. Vincular el deployer SA con el pool

```bash
WIF_POOL=$(gcloud iam workload-identity-pools describe github-pool \
  --project $PROJECT_ID \
  --location global \
  --format "value(name)")

gcloud iam service-accounts add-iam-policy-binding \
  protou-deployer@$PROJECT_ID.iam.gserviceaccount.com \
  --project $PROJECT_ID \
  --role roles/iam.workloadIdentityUser \
  --member "principalSet://iam.googleapis.com/${WIF_POOL}/attribute.repository/${GITHUB_ORG}/${REPO_NAME}"
```

---

## 5. Configurar secrets en GitHub

Ir a **GitHub → repo → Settings → Secrets and variables → Actions → New repository secret**.

| Secret | Valor |
|--------|-------|
| `GCP_PROJECT_ID` | tu project ID (ej: `protou-abc123`) |
| `GCP_REGION` | `us-central1` |
| `WIF_PROVIDER` | output del paso 3 (ej: `projects/123.../providers/github-provider`) |
| `WIF_SERVICE_ACCOUNT` | `protou-deployer@<PROJECT_ID>.iam.gserviceaccount.com` |
| `API_SERVICE_ACCOUNT` | `protou-api@<PROJECT_ID>.iam.gserviceaccount.com` |

---

## 6. Verificar que funciona

Hacer cualquier push a `main` y revisar:
- GitHub Actions → `Deploy API` → debe completar sin error
- GitHub Actions → `Deploy Scraper` → debe completar sin error

```bash
# Verificar que el API está corriendo
gcloud run services describe protou-api \
  --region us-central1 \
  --format "value(status.url)"

# Verificar que el Job del scraper tiene la imagen nueva
gcloud run jobs describe protou-scraper \
  --region us-central1 \
  --format "value(spec.template.spec.template.spec.containers[0].image)"
```

---

## Flujo completo después del setup

```
Push a main
  ├── CI (ci.yml)
  │   ├── lint
  │   ├── test (unit + integración contra postgres)
  │   └── [sin build-check — solo en PRs]
  │
  ├── Deploy API (deploy-api.yml) — en paralelo con CI
  │   ├── build + push api:SHA y api:latest
  │   ├── build + push migrate:SHA
  │   ├── actualizar migrate job
  │   ├── ejecutar migraciones (--wait)
  │   ├── deploy Cloud Run Service
  │   └── smoke test /health
  │
  └── Deploy Scraper (deploy-scraper.yml) — en paralelo con CI
      ├── build + push scraper:SHA y scraper:latest
      └── actualizar Cloud Run Job

Push a PR (no merge)
  ├── lint
  ├── test
  └── build-check (sin push)
```

> **Nota:** Deploy API y Deploy Scraper corren en paralelo con CI al hacer push a main.
> Si el CI falla, los deploys ya corrieron — esto es aceptable porque `main` siempre debe
> estar en estado deployable. Si preferís que el deploy espere al CI, mover los workflows
> a un único archivo con `needs: [lint, test]` en los jobs de deploy.

---

## Rollback

```bash
# Ver revisiones anteriores del API
gcloud run revisions list --service protou-api --region us-central1

# Apuntar todo el tráfico a una revisión anterior
gcloud run services update-traffic protou-api \
  --region us-central1 \
  --to-revisions protou-api-XXXXXXX=100

# Rollback del scraper (volver a imagen anterior)
gcloud run jobs update protou-scraper \
  --image us-central1-docker.pkg.dev/<PROJECT_ID>/protou/scraper:<SHA_ANTERIOR> \
  --region us-central1
```
