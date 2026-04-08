//go:build !integration

package operator

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// NOTE: DeliveryConfigHandler.updateConfig and getConfig both require a real DB
// (BeginTx, QueryRowContext, ExecContext). The bracket validation logic is inline
// in updateConfig and cannot be exercised without a DB connection for the full handler.
//
// However, we can test the HTTP-level validation rejection (fee_cop <= 0) by calling
// updateConfig via httptest — the handler rejects invalid brackets BEFORE touching the
// DB, so we can verify the early-return path returns 400 even with a nil-DB handler
// (the nil-DB panic only occurs if we reach the BeginTx call).
//
// Tests that reach DB calls are marked below and would require integration setup.

// newDeliveryConfigHandlerNoDB creates a handler with nil DB.
// Only safe for tests that exercise code paths that never call h.db.
func newDeliveryConfigHandlerNoDB() *DeliveryConfigHandler {
	return &DeliveryConfigHandler{db: nil}
}

// ─── bracket validation (pure HTTP early-return, no DB required) ─────────────

func TestUpdateConfig_NegativeFee_Returns400(t *testing.T) {
	h := newDeliveryConfigHandlerNoDB()

	reqBody := map[string]interface{}{
		"brackets": []map[string]interface{}{
			{"distance_km_min": 0, "distance_km_max": 5.0, "fee_cop": -500},
		},
		"multi_store_discount_pct": 30,
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPut, "/v1/operator/delivery-config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.updateConfig(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code,
		"negative fee_cop should be rejected with 400")

	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "INVALID_FIELDS", resp["code"])
}

func TestUpdateConfig_ZeroFee_Returns400(t *testing.T) {
	h := newDeliveryConfigHandlerNoDB()

	reqBody := map[string]interface{}{
		"brackets": []map[string]interface{}{
			{"distance_km_min": 0, "distance_km_max": 5.0, "fee_cop": 0},
		},
		"multi_store_discount_pct": 30,
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPut, "/v1/operator/delivery-config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.updateConfig(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code,
		"zero fee_cop must be rejected (fee_cop must be > 0)")

	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "INVALID_FIELDS", resp["code"])
}

func TestUpdateConfig_InvalidBody_Returns400(t *testing.T) {
	h := newDeliveryConfigHandlerNoDB()

	req := httptest.NewRequest(http.MethodPut, "/v1/operator/delivery-config",
		bytes.NewReader([]byte("not-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.updateConfig(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, "INVALID_BODY", resp["code"])
}

// TestUpdateConfig_MultipleBrackets_OneBadFee verifies that a single negative fee in a
// multi-bracket payload still triggers validation rejection (first-bad-bracket wins).
func TestUpdateConfig_MultipleBrackets_OneBadFee_Returns400(t *testing.T) {
	h := newDeliveryConfigHandlerNoDB()

	reqBody := map[string]interface{}{
		"brackets": []map[string]interface{}{
			{"distance_km_min": 0, "distance_km_max": 5.0, "fee_cop": 3000},  // valid
			{"distance_km_min": 5, "distance_km_max": 15.0, "fee_cop": -100}, // invalid
		},
		"multi_store_discount_pct": 30,
	}
	body, err := json.Marshal(reqBody)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPut, "/v1/operator/delivery-config", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.updateConfig(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code,
		"one invalid bracket among multiple should still reject with 400")
}

// ─── DB-dependent paths (documented, not run as unit tests) ──────────────────
//
// The following paths in DeliveryConfigHandler require a real DB and are covered
// by integration tests (build tag: integration):
//
//   - updateConfig happy path: valid brackets → BeginTx → DELETE → INSERT → UPSERT → Commit
//   - getConfig: QueryRowContext on delivery_fee_brackets and delivery_config tables
//   - fetchBrackets: rows.Scan on delivery_fee_brackets
