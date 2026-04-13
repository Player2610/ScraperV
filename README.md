# protou

Plataforma intermediaria (dropshipping) de componentes electrónicos para estudiantes universitarios en Bogotá.

## Desarrollo local

### Prerrequisitos

- **Go 1.24+** (el módulo usa `go 1.23`, toolchain `go1.24`)
- **Docker + Docker Compose v2** — backend + DB en contenedores
- **Bun 1.3+** — gestor de paquetes del frontend (`packageManager: bun@1.3.11`)
- **golangci-lint** — opcional, solo para `make lint`

---

### Entornos: dev vs prod

| | Dev | Prod |
|-|-----|------|
| **DB** | pgvector en Docker (volumen local) | Neon (PostgreSQL serverless) |
| **API** | Docker container | GCP Cloud Run |
| **Frontend** | `bun dev` en host | Cloudflare Pages |
| **Datos** | Sincronizados desde prod al arrancar | Fuente de verdad |

> Los datos solo fluyen **prod → dev**. Nunca al revés.

---

### 1. Variables de entorno

```bash
cp .env.example .env
# Edita .env y completa DATABASE_URL (Docker local), PROD_DATABASE_URL (Neon), JWT_SECRET, RESEND_API_KEY
```

Variables de `.env`:

| Variable | Requerida | Descripción |
|----------|-----------|-------------|
| `DATABASE_URL` | Sí | `postgres://postgres:password@localhost:5432/protou?sslmode=disable` |
| `PROD_DATABASE_URL` | Sí | Connection string de Neon — fuente del sync |
| `JWT_SECRET` | Sí | Mínimo 32 caracteres |
| `RESEND_API_KEY` | Sí | Clave Resend para emails |
| `GOOGLE_MAPS_API_KEY` | No | Geocodificación (opcional — degrada sin él) |
| `NOTIFICATIONS_FROM` | No | Remitente de emails (default: `protou DEV <pedidos+dev@protou.co>`) |
| `CORS_ORIGIN` | No | Default: `http://localhost:4321` |
| `ALERT_EMAIL` | No | Email para alertas del scraper (dejar en blanco para desactivar) |

```bash
# web/.env — URL del API para el frontend en dev
cp web/.env.example web/.env
```

---

### 2. Levantar el stack de desarrollo

```bash
make dev
```

Esto ejecuta `docker compose up --build` con la siguiente secuencia:

```
db (pgvector:pg16)
  └── db-sync  ← pg_dump de Neon → pg_restore local  [one-shot]
        └── migrate  ← migraciones pendientes locales  [one-shot]
              └── api  ← escucha en :8080
```

Al terminar, el API tiene exactamente los datos de producción.

**Variantes:**

```bash
make dev-quick   # reinicia solo el API sin re-sincronizar (la DB ya está corriendo)
make dev-reset   # borra el volumen y sincroniza desde cero
```

---

### 3. Levantar el frontend

```bash
cd web
bun install      # primera vez
bun dev          # http://localhost:4321
```

El frontend lee `web/.env` → `PUBLIC_API_URL=http://localhost:8080`.

---

### 4. Verificar que todo funciona

```bash
# Health check del API
curl -s http://localhost:8080/health | jq
# {"db":"ok","status":"ok"}
```

Abre `http://localhost:4321` en el browser.

**Panel del operador** → `http://localhost:4321/operator/login`
- Email: `operator@protou.co`
- Contraseña: `operator123`

---

### 5. Migraciones

```bash
make migrate-up    # aplica migraciones pendientes (usa DATABASE_URL de .env)
make migrate-down  # revierte la última migración
```

> Las migraciones se corren automáticamente dentro del contenedor al hacer `make dev`.
> `make migrate-up` en host es útil para aplicar migraciones a Neon (prod) directamente desde `.env`.

---

### 6. Scraper

```bash
make scraper-dry-run   # parsea y muestra listings sin escribir a DB
go run ./cmd/scraper   # ingestar datos reales (requiere .env cargado)
```

---

### 7. Tests

```bash
make test             # tests unitarios con race detector
make test-integration # tests de integración (requiere TEST_DATABASE_URL)
make lint             # golangci-lint
```
