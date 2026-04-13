# Discovery: Tests Review — Hardening Complete

> Engram #118 | 2026-04-08 | topic: `tests/hardening-review`

**What**: Reviewed all unit tests after hardening phase (Phases 0-5). Found no broken tests — all existing tests already matched the new error codes and behaviors. Added three new test files covering gaps.

**Why**: Hardening added: cart quantity validation (1-99), new `DELETE /v1/cart/items/{listing_id}` endpoint, email/password validation with `INVALID_EMAIL`/`PASSWORD_TOO_SHORT`, `INVALID_PAYMENT_METHOD` validation, `SecurityHeaders` middleware, health endpoint DB status.

**Gaps found and covered**:

1. **No cart handler tests at all** — added `internal/cart/handler_test.go` covering:
   - Auth guard (401) on all 5 endpoints
   - `INVALID_QUANTITY` for qty=0, negative, >99
   - `INVALID_PARAM` for non-integer listing_id
   - `INVALID_BODY` for malformed JSON
   - Route existence check for new `DELETE /v1/cart/items/{listing_id}`

2. **No orders handler tests at all** — added `internal/orders/handler_test.go` covering:
   - Auth guard (401) on all 4 endpoints
   - `MISSING_FIELDS` for empty payment_method
   - `INVALID_PAYMENT_METHOD` for unrecognised values
   - Valid payment methods pass validation (nequi, daviplata, efectivo, llaves_breve)
   - `INVALID_BODY` for malformed JSON

3. **No healthHandler tests** — added to `internal/platform/httpserver_test.go`:
   - DB ok → 200 `{"status":"ok","db":"ok"}` using sqlmock with `MonitorPingsOption`
   - DB down → 503 `{"status":"degraded","db":"error","error":"..."}`

**Learned**:
- `go-sqlmock` requires `sqlmock.MonitorPingsOption(true)` to intercept `PingContext` calls — without it, `ExpectPing()` is ignored and pings always succeed.
- Cart handler tests use `nil` service with a `recover()` wrapper for paths that reach the service layer.
- All existing tests correctly used the new error codes — no tests were broken.
- **Final `go test ./...` result**: All 11 packages pass (0 failures).
