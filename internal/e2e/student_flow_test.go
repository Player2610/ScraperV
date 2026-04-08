//go:build integration

package e2e_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/protou/protou/internal/cart"
	"github.com/protou/protou/internal/catalog"
	"github.com/protou/protou/internal/delivery"
	"github.com/protou/protou/internal/notifications"
	"github.com/protou/protou/internal/orders"
	"github.com/protou/protou/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openE2EDB opens a real DB connection for E2E integration tests.
// Skips if TEST_DATABASE_URL is not set.
func openE2EDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping E2E integration tests")
	}
	db, err := sql.Open("postgres", url)
	require.NoError(t, err)
	require.NoError(t, db.PingContext(context.Background()))
	return db
}

// uniqueSuffix returns a short time-based suffix for test data isolation.
func uniqueSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// noopNotifSvc returns a notifications.Service backed by a real DB but with no
// email sending (the service silently logs failures — no Resend API key is set
// in test environments).
func noopNotifSvc(db *sql.DB) *notifications.Service {
	return notifications.NewService(db)
}

// TestStudentFlow_RegisterLoginCartCheckout verifies the full student flow:
// register → login → add to cart → get cart → delivery fee calculation.
//
// Since the services use concrete *Repository types (not interfaces), this test
// drives the service and repository layers directly against a real DB — not HTTP.
// This matches the pattern in internal/scraping/integration_test.go.
//
// Limitations:
//   - Cart AddToCart requires a real listing in the DB. If none exists, the test
//     verifies that the expected error (ErrUnavailable) is returned — that the
//     guard works correctly — and skips the full checkout path.
//   - Delivery fee calculation requires a real address with a valid addressID.
func TestStudentFlow_RegisterLoginCartCheckout(t *testing.T) {
	db := openE2EDB(t)
	defer db.Close()

	ctx := context.Background()
	suffix := uniqueSuffix()
	testEmail := fmt.Sprintf("e2e_student_%s@example.com", suffix)
	testPassword := "securepassword123"
	testName := "E2E Test Student"

	// ── Setup: initialize repositories and services ───────────────────────────
	userRepo := users.NewRepository(db)
	userSvc := users.NewService(userRepo)

	cartRepo := cart.NewRepository(db)
	catalogRepo := catalog.NewRepository(db)
	cartSvc := cart.NewService(cartRepo, catalogRepo)

	deliveryRepo := delivery.NewRepository(db)
	orderRepo := orders.NewRepository(db)
	orderSvc := orders.NewService(orderRepo, cartSvc, catalogRepo, deliveryRepo, userRepo, noopNotifSvc(db))

	// ── Step 1: Register ───────────────────────────────────────────────────────
	req := users.RegisterRequest{
		Email:             testEmail,
		Phone:             "3001234567",
		FullName:          testName,
		Password:          testPassword,
		HabeasDataConsent: true,
	}
	user, token, err := userSvc.Register(ctx, req)
	require.NoError(t, err, "register should succeed")
	require.NotNil(t, user)
	assert.NotEmpty(t, token, "JWT token should be non-empty")
	assert.Equal(t, testEmail, user.Email)
	assert.NotNil(t, user.HabeasDataConsentAt, "habeas_data_consent_at should be set when consent=true")

	// Cleanup: anonymize the test user after test (HABEAS DATA-compliant cleanup)
	t.Cleanup(func() {
		if err := userSvc.DeleteAccount(context.Background(), user.ID); err != nil {
			t.Logf("cleanup: failed to anonymize user %d: %v", user.ID, err)
		}
		// Remove any carts and orders for this user
		_, _ = db.ExecContext(context.Background(),
			`DELETE FROM cart_items WHERE cart_id IN (SELECT id FROM carts WHERE user_id=$1)`, user.ID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM carts WHERE user_id=$1`, user.ID)
	})

	// ── Step 2: Login ─────────────────────────────────────────────────────────
	loginUser, loginToken, err := userSvc.Login(ctx, testEmail, testPassword)
	require.NoError(t, err, "login should succeed with correct credentials")
	require.NotNil(t, loginUser)
	assert.NotEmpty(t, loginToken, "login JWT token should be non-empty")
	assert.Equal(t, user.ID, loginUser.ID, "login should return the same user")

	// Verify wrong password is rejected
	_, _, err = userSvc.Login(ctx, testEmail, "wrongpassword")
	assert.ErrorIs(t, err, users.ErrInvalidCredentials, "wrong password should return ErrInvalidCredentials")

	// ── Step 3: Add to cart ────────────────────────────────────────────────────
	// Look for any available active listing in the DB to use as test data.
	var testListingID int64
	err = db.QueryRowContext(ctx,
		`SELECT id FROM listings WHERE is_active=true AND stock_signal='in_stock' AND price_cop IS NOT NULL LIMIT 1`,
	).Scan(&testListingID)

	if err == sql.ErrNoRows {
		// No listings in the test DB — verify cart AddToCart returns ErrUnavailable
		// for a non-existent listing (the error guard works).
		t.Log("No active listings in test DB; testing ErrUnavailable guard")
		addErr := cartSvc.AddToCart(ctx, user.ID, 999999999, 1)
		assert.ErrorIs(t, addErr, cart.ErrUnavailable,
			"non-existent listing should return ErrUnavailable")

		// Verify cart is empty
		cartResp, getErr := cartSvc.GetCart(ctx, user.ID)
		require.NoError(t, getErr)
		assert.Empty(t, cartResp.Items, "cart should be empty when no listings exist")
		return
	}
	require.NoError(t, err, "listing lookup should not error")

	// Add item to cart
	err = cartSvc.AddToCart(ctx, user.ID, testListingID, 2)
	require.NoError(t, err, "adding active listing to cart should succeed")

	// ── Step 4: Get cart ───────────────────────────────────────────────────────
	cartResp, err := cartSvc.GetCart(ctx, user.ID)
	require.NoError(t, err, "getting cart should succeed")
	require.NotNil(t, cartResp.Cart)
	require.Len(t, cartResp.Items, 1, "cart should have exactly one item")
	assert.Equal(t, testListingID, cartResp.Items[0].CartItem.ListingID, "cart item listing ID should match")
	assert.Equal(t, 2, cartResp.Items[0].CartItem.Quantity, "cart item quantity should be 2")

	// Update quantity (upsert)
	err = cartSvc.AddToCart(ctx, user.ID, testListingID, 3)
	require.NoError(t, err, "updating cart quantity should succeed")

	cartResp2, err := cartSvc.GetCart(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, cartResp2.Items, 1)
	assert.Equal(t, 3, cartResp2.Items[0].CartItem.Quantity, "quantity should be updated to 3")

	// Remove item (qty=0)
	err = cartSvc.AddToCart(ctx, user.ID, testListingID, 0)
	require.NoError(t, err, "removing item with qty=0 should succeed")

	cartResp3, err := cartSvc.GetCart(ctx, user.ID)
	require.NoError(t, err)
	assert.Empty(t, cartResp3.Items, "cart should be empty after qty=0 removal")

	// Re-add for delivery fee test
	err = cartSvc.AddToCart(ctx, user.ID, testListingID, 1)
	require.NoError(t, err)

	// ── Step 5: Delivery fee calculation ──────────────────────────────────────
	// Create a test address for the user in Bogotá.
	testAddr := users.AddressInput{
		FullAddress: "Calle 45 # 10-20, Chapinero, Bogotá",
	}
	addr, err := userSvc.AddAddress(ctx, user.ID, testAddr)
	require.NoError(t, err, "adding address should succeed")
	assert.NotZero(t, addr.ID)

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM addresses WHERE id=$1`, addr.ID)
	})

	feeResult, err := orderSvc.CalculateDeliveryFee(ctx, user.ID, addr.ID)
	require.NoError(t, err, "delivery fee calculation should succeed for Bogotá address")
	assert.Greater(t, feeResult.TotalFee, 0, "delivery fee should be positive for Bogotá")

	// Out-of-zone address should be rejected
	outZoneAddr, err := userSvc.AddAddress(ctx, user.ID, users.AddressInput{
		FullAddress: "Calle 10 # 5-20, Medellín, Antioquia",
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM addresses WHERE id=$1`, outZoneAddr.ID)
	})

	_, err = orderSvc.CalculateDeliveryFee(ctx, user.ID, outZoneAddr.ID)
	assert.ErrorIs(t, err, orders.ErrOutsideZone, "out-of-zone address should return ErrOutsideZone")
}

// TestStudentFlow_DuplicateEmail verifies that registering with a duplicate email
// returns ErrDuplicateEmail.
func TestStudentFlow_DuplicateEmail(t *testing.T) {
	db := openE2EDB(t)
	defer db.Close()

	ctx := context.Background()
	suffix := uniqueSuffix()
	testEmail := fmt.Sprintf("e2e_dup_%s@example.com", suffix)

	userRepo := users.NewRepository(db)
	userSvc := users.NewService(userRepo)

	req := users.RegisterRequest{
		Email:             testEmail,
		FullName:          "Duplicate Test",
		Password:          "password123",
		HabeasDataConsent: true,
	}

	user, _, err := userSvc.Register(ctx, req)
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := userSvc.DeleteAccount(context.Background(), user.ID); err != nil {
			t.Logf("cleanup: %v", err)
		}
	})

	// Second registration with same email must fail
	_, _, err = userSvc.Register(ctx, req)
	assert.ErrorIs(t, err, users.ErrDuplicateEmail, "duplicate email should return ErrDuplicateEmail")
}
