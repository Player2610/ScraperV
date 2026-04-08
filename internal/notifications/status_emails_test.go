//go:build !integration

package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturedRequest holds what the fake Resend endpoint received.
type capturedRequest struct {
	Subject string   `json:"subject"`
	To      []string `json:"to"`
	From    string   `json:"from"`
}

// newStatusTestServer creates a test HTTP server that returns the given status code
// and records the last parsed request body into *capturedRequest (may be nil if caller
// does not need inspection).
func newStatusTestServer(t *testing.T, statusCode int, captured *capturedRequest) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if captured != nil {
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			_ = json.Unmarshal(body, captured)
		}
		w.WriteHeader(statusCode)
	}))
}

// ─── SendOrderConfirmed ──────────────────────────────────────────────────────

func TestSendOrderConfirmed_HTTP200_ReturnsNil(t *testing.T) {
	srv := newStatusTestServer(t, http.StatusOK, nil)
	defer srv.Close()
	t.Setenv("RESEND_API_KEY", "test-key")

	svc := newTestService(srv.URL)
	err := svc.SendOrderConfirmed(context.Background(), "u@test.co", "Ana", 42)

	assert.NoError(t, err, "SendOrderConfirmed must return nil on HTTP 200 (fire-and-forget)")
}

func TestSendOrderConfirmed_HTTP500_ReturnsNil(t *testing.T) {
	srv := newStatusTestServer(t, http.StatusInternalServerError, nil)
	defer srv.Close()
	t.Setenv("RESEND_API_KEY", "test-key")

	svc := newTestService(srv.URL)
	err := svc.SendOrderConfirmed(context.Background(), "u@test.co", "Ana", 7)

	assert.NoError(t, err, "SendOrderConfirmed must return nil even on HTTP 500 (fire-and-forget)")
}

func TestSendOrderConfirmed_SubjectContainsOrderID(t *testing.T) {
	var captured capturedRequest
	srv := newStatusTestServer(t, http.StatusOK, &captured)
	defer srv.Close()
	t.Setenv("RESEND_API_KEY", "test-key")

	svc := newTestService(srv.URL)
	orderID := int64(123)
	err := svc.SendOrderConfirmed(context.Background(), "u@test.co", "Ana", orderID)

	require.NoError(t, err)
	assert.Contains(t, captured.Subject, fmt.Sprintf("%d", orderID),
		"subject should contain the order ID")
}

// ─── SendOrderInDelivery ─────────────────────────────────────────────────────

func TestSendOrderInDelivery_ReturnsNil(t *testing.T) {
	srv := newStatusTestServer(t, http.StatusOK, nil)
	defer srv.Close()
	t.Setenv("RESEND_API_KEY", "test-key")

	svc := newTestService(srv.URL)
	err := svc.SendOrderInDelivery(context.Background(), "u@test.co", "Luis", 55)

	assert.NoError(t, err)
}

func TestSendOrderInDelivery_SubjectContainsOrderID(t *testing.T) {
	var captured capturedRequest
	srv := newStatusTestServer(t, http.StatusOK, &captured)
	defer srv.Close()
	t.Setenv("RESEND_API_KEY", "test-key")

	svc := newTestService(srv.URL)
	orderID := int64(456)
	err := svc.SendOrderInDelivery(context.Background(), "u@test.co", "Luis", orderID)

	require.NoError(t, err)
	assert.Contains(t, captured.Subject, fmt.Sprintf("%d", orderID),
		"subject should contain the order ID")
}

// ─── SendOrderDelivered ──────────────────────────────────────────────────────

func TestSendOrderDelivered_ReturnsNil(t *testing.T) {
	srv := newStatusTestServer(t, http.StatusOK, nil)
	defer srv.Close()
	t.Setenv("RESEND_API_KEY", "test-key")

	svc := newTestService(srv.URL)
	err := svc.SendOrderDelivered(context.Background(), "u@test.co", "Maria", 77)

	assert.NoError(t, err)
}

func TestSendOrderDelivered_SubjectContainsOrderID(t *testing.T) {
	var captured capturedRequest
	srv := newStatusTestServer(t, http.StatusOK, &captured)
	defer srv.Close()
	t.Setenv("RESEND_API_KEY", "test-key")

	svc := newTestService(srv.URL)
	orderID := int64(789)
	err := svc.SendOrderDelivered(context.Background(), "u@test.co", "Maria", orderID)

	require.NoError(t, err)
	assert.Contains(t, captured.Subject, fmt.Sprintf("%d", orderID),
		"subject should contain the order ID")
}

// ─── SendOrderCancelled ──────────────────────────────────────────────────────

func TestSendOrderCancelled_ReturnsNil(t *testing.T) {
	srv := newStatusTestServer(t, http.StatusOK, nil)
	defer srv.Close()
	t.Setenv("RESEND_API_KEY", "test-key")

	svc := newTestService(srv.URL)
	err := svc.SendOrderCancelled(context.Background(), "u@test.co", "Pedro", 99)

	assert.NoError(t, err)
}

func TestSendOrderCancelled_SubjectContainsOrderID(t *testing.T) {
	var captured capturedRequest
	srv := newStatusTestServer(t, http.StatusOK, &captured)
	defer srv.Close()
	t.Setenv("RESEND_API_KEY", "test-key")

	svc := newTestService(srv.URL)
	orderID := int64(321)
	err := svc.SendOrderCancelled(context.Background(), "u@test.co", "Pedro", orderID)

	require.NoError(t, err)
	assert.Contains(t, captured.Subject, fmt.Sprintf("%d", orderID),
		"subject should contain the order ID")
}

// ─── Subject format table test ───────────────────────────────────────────────

// TestStatusEmailSubjectFormats verifies every status email function produces
// a subject that includes the order ID and "protou" branding.
func TestStatusEmailSubjectFormats(t *testing.T) {
	type sendFn func(svc *Service, orderID int64) error

	tests := []struct {
		name string
		fn   sendFn
	}{
		{
			name: "SendOrderConfirmed",
			fn: func(svc *Service, orderID int64) error {
				return svc.SendOrderConfirmed(context.Background(), "u@test.co", "User", orderID)
			},
		},
		{
			name: "SendOrderInDelivery",
			fn: func(svc *Service, orderID int64) error {
				return svc.SendOrderInDelivery(context.Background(), "u@test.co", "User", orderID)
			},
		},
		{
			name: "SendOrderDelivered",
			fn: func(svc *Service, orderID int64) error {
				return svc.SendOrderDelivered(context.Background(), "u@test.co", "User", orderID)
			},
		},
		{
			name: "SendOrderCancelled",
			fn: func(svc *Service, orderID int64) error {
				return svc.SendOrderCancelled(context.Background(), "u@test.co", "User", orderID)
			},
		},
	}

	orderID := int64(1001)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured capturedRequest
			srv := newStatusTestServer(t, http.StatusOK, &captured)
			defer srv.Close()
			t.Setenv("RESEND_API_KEY", "test-key")

			svc := newTestService(srv.URL)
			err := tt.fn(svc, orderID)

			require.NoError(t, err)
			assert.Contains(t, captured.Subject, fmt.Sprintf("%d", orderID),
				"%s: subject should contain order ID %d", tt.name, orderID)
			assert.Contains(t, captured.Subject, "protou",
				"%s: subject should contain protou branding", tt.name)
		})
	}
}

// ─── fire-and-forget behavior ────────────────────────────────────────────────

// TestStatusEmails_NoAPIKey_NeverError ensures fire-and-forget: when RESEND_API_KEY is
// absent, all status email functions must return nil (they log and skip, never error).
func TestStatusEmails_NoAPIKey_NeverError(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "")
	svc := &Service{db: nil, tmpl: loadTemplate(), httpClient: http.DefaultClient}

	assert.NoError(t, svc.SendOrderConfirmed(context.Background(), "x@x.co", "U", 1))
	assert.NoError(t, svc.SendOrderInDelivery(context.Background(), "x@x.co", "U", 2))
	assert.NoError(t, svc.SendOrderDelivered(context.Background(), "x@x.co", "U", 3))
	assert.NoError(t, svc.SendOrderCancelled(context.Background(), "x@x.co", "U", 4))
}
