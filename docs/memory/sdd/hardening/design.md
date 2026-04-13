# Design: Fase 5 — Hardening

> Engram #114 | topic: `sdd/hardening/design` | 2026-04-07

_Versión condensada. Ver engram ID #114 para el documento completo con diagramas de flujo._

---

## Enfoque General

Hardening como cambios ortogonales sin nueva abstracción de dominio:
- (A) Refactorizar `cmd/api/main.go` → `http.Server` real con timeouts y signal handling
- (B) Insertar middlewares de seguridad en cadena chi
- (C) Validar inputs en handlers existentes
- (D) HABEAS DATA con migración additive + nuevo endpoint
- (E) Migrar logging a `log/slog` con JSONHandler
- (F) Tests E2E contra DB real con build tag `integration`

---

## Decisiones Clave

| Decisión | Elegida | Descartada | Razón |
|----------|---------|-----------|-------|
| Rate limiting | `go-chi/httprate` | `x/time/rate` manual | Drop-in chi middleware |
| Input validation | Inline en handlers | `go-playground/validator` | Sobreingeniería para 5 validaciones |
| Email validation | `regexp` RFC5322 simplificado | `net/mail.ParseAddress` | net/mail acepta "Nombre <email>" |
| Eliminación HABEAS DATA | Anonimización (soft delete) | Hard delete | Preserva integridad referencial |
| Logging | `log/slog` stdlib | zerolog, zap | Cero dependencias nuevas |
| E2E test DB | DB real via `DATABASE_URL` | testcontainers | Sin Docker en CI |

---

## Secuencia de Implementación

1. **H1+H5+H3+H7** (un commit) — main.go + httpserver.go
2. **H2+H4** (un commit) — handlers
3. **H6** (un commit) — migración + users domain + web
4. **H8–H10** (un commit) — solo tests

---

## Interfaces Nuevas / Cambiadas

### NewServer
```go
func NewServer(cfg Config, db *sql.DB) *chi.Mux  // db para health check
```

### RegisterRequest
```go
type RegisterRequest struct {
    ...
    HabeasDataConsent bool `json:"habeas_data_consent"`
}
```

### DELETE /v1/users/me → 202
```json
{"message":"Tu cuenta será eliminada en las próximas 24 horas"}
```

### Health check
```json
{"status":"ok","db":"ok"}        // 200
{"status":"degraded","db":"error"} // 503
```
