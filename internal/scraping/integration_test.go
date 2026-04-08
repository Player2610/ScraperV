//go:build integration

package scraping_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/protou/protou/internal/scraping"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openTestDB opens a connection to the test database.
// If TEST_DATABASE_URL is not set the test is skipped.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set — skipping integration tests")
	}
	db, err := sql.Open("postgres", url)
	require.NoError(t, err)
	require.NoError(t, db.PingContext(context.Background()))
	return db
}

// insertTestStore creates a store and scrape_rule in the DB, returning the store ID.
func insertTestStore(t *testing.T, db *sql.DB, name, baseURL, catalogPattern string) int64 {
	t.Helper()

	var storeID int64
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO stores (name, base_url, lat, lng, is_active) VALUES ($1, $2, 4.628, -74.064, true) RETURNING id`,
		name, baseURL,
	).Scan(&storeID)
	require.NoError(t, err)

	_, err = db.ExecContext(context.Background(), `
		INSERT INTO scrape_rules (
			store_id, catalog_url_pattern,
			item_selector, price_selector, name_selector,
			image_selector, stock_selector, pagination_selector,
			delay_ms
		) VALUES (
			$1, $2,
			'ul.products li.product',
			'span.woocommerce-Price-amount bdi',
			'h2.woocommerce-loop-product__title',
			'img.wp-post-image',
			'.out-of-stock',
			'a.next.page-numbers',
			0
		)`,
		storeID, catalogPattern,
	)
	require.NoError(t, err)

	return storeID
}

// cleanupStore removes all data created for the test store.
func cleanupStore(t *testing.T, db *sql.DB, storeID int64) {
	t.Helper()
	_, _ = db.ExecContext(context.Background(), `DELETE FROM price_history WHERE listing_id IN (SELECT id FROM listings WHERE store_id=$1)`, storeID)
	_, _ = db.ExecContext(context.Background(), `DELETE FROM listings WHERE store_id=$1`, storeID)
	_, _ = db.ExecContext(context.Background(), `DELETE FROM scrape_jobs WHERE store_id=$1`, storeID)
	_, _ = db.ExecContext(context.Background(), `DELETE FROM scrape_rules WHERE store_id=$1`, storeID)
	_, _ = db.ExecContext(context.Background(), `DELETE FROM stores WHERE id=$1`, storeID)
}

// serveFixturePage returns an httptest.Server that serves the given HTML for all requests.
func serveFixturePage(html string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, html)
	}))
}

const testFixtureHTML = `<!DOCTYPE html>
<html><body>
<ul class="products">
  <li class="product">
    <a href="/produto/item-a/" class="woocommerce-LoopProduct-link">
      <img class="wp-post-image" src="/img/a.jpg">
      <h2 class="woocommerce-loop-product__title">Item A</h2>
      <span class="woocommerce-Price-amount"><bdi>$ 1.000</bdi></span>
    </a>
  </li>
  <li class="product">
    <a href="/produto/item-b/" class="woocommerce-LoopProduct-link">
      <img class="wp-post-image" src="/img/b.jpg">
      <h2 class="woocommerce-loop-product__title">Item B</h2>
      <span class="woocommerce-Price-amount"><bdi>$ 2.500</bdi></span>
    </a>
  </li>
</ul>
</body></html>`

const testFixtureHTMLUpdatedPrice = `<!DOCTYPE html>
<html><body>
<ul class="products">
  <li class="product">
    <a href="/produto/item-a/" class="woocommerce-LoopProduct-link">
      <img class="wp-post-image" src="/img/a.jpg">
      <h2 class="woocommerce-loop-product__title">Item A</h2>
      <span class="woocommerce-Price-amount"><bdi>$ 1.500</bdi></span>
    </a>
  </li>
  <li class="product">
    <a href="/produto/item-b/" class="woocommerce-LoopProduct-link">
      <img class="wp-post-image" src="/img/b.jpg">
      <h2 class="woocommerce-loop-product__title">Item B</h2>
      <span class="woocommerce-Price-amount"><bdi>$ 2.500</bdi></span>
    </a>
  </li>
</ul>
</body></html>`

func TestRunStore_FullFlow(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()

	srv := serveFixturePage(testFixtureHTML)
	defer srv.Close()

	storeID := insertTestStore(t, db, "Test Store Integration", srv.URL, srv.URL+"/?page={page}")
	defer cleanupStore(t, db, storeID)

	notifier := &scraping.NoopNotifier{}
	worker := scraping.NewWorker(db, &scraping.DefaultParser{}, notifier)
	runner := scraping.NewRunner(db, worker)

	// --- First scrape: both listings should be new ---
	err := runner.RunStore(ctx, storeID)
	require.NoError(t, err)

	var listingCount int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM listings WHERE store_id=$1`, storeID,
	).Scan(&listingCount))
	assert.Equal(t, 2, listingCount, "first scrape should create 2 listings")

	var job1Found, job1New int
	var job1Status string
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT status, listings_found, listings_new FROM scrape_jobs WHERE store_id=$1 ORDER BY started_at DESC LIMIT 1`,
		storeID,
	).Scan(&job1Status, &job1Found, &job1New))
	assert.Equal(t, "success", job1Status)
	assert.Equal(t, 2, job1Found)
	assert.Equal(t, 2, job1New)

	// --- Second scrape (same fixture, no changes): no new listings, no price_history ---
	var priceHistoryBefore int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM price_history WHERE listing_id IN (SELECT id FROM listings WHERE store_id=$1)`,
		storeID,
	).Scan(&priceHistoryBefore))

	time.Sleep(10 * time.Millisecond) // ensure started_at ordering
	err = runner.RunStore(ctx, storeID)
	require.NoError(t, err)

	var listingCountAfter2 int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM listings WHERE store_id=$1`, storeID,
	).Scan(&listingCountAfter2))
	assert.Equal(t, 2, listingCountAfter2, "second scrape should not add new listings")

	var priceHistoryAfter2 int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM price_history WHERE listing_id IN (SELECT id FROM listings WHERE store_id=$1)`,
		storeID,
	).Scan(&priceHistoryAfter2))
	assert.Equal(t, priceHistoryBefore, priceHistoryAfter2, "second scrape with no price change should not add price_history rows")

	// --- Third scrape (updated price for Item A): price_history should gain one row ---
	// Replace server handler to return updated price
	updatedFixture := strings.NewReplacer().Replace(testFixtureHTMLUpdatedPrice)
	srv2 := serveFixturePage(updatedFixture)
	defer srv2.Close()

	// Update store catalog URL to new server
	_, err = db.ExecContext(ctx,
		`UPDATE scrape_rules SET catalog_url_pattern=$1 WHERE store_id=$2`,
		srv2.URL+"/?page={page}", storeID,
	)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	err = runner.RunStore(ctx, storeID)
	require.NoError(t, err)

	var priceHistoryAfter3 int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM price_history WHERE listing_id IN (SELECT id FROM listings WHERE store_id=$1)`,
		storeID,
	).Scan(&priceHistoryAfter3))
	assert.Greater(t, priceHistoryAfter3, priceHistoryAfter2,
		"price change on Item A should produce a new price_history row")

	// --- Verify last job status ---
	var finalStatus string
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT status FROM scrape_jobs WHERE store_id=$1 ORDER BY started_at DESC LIMIT 1`,
		storeID,
	).Scan(&finalStatus))
	assert.Equal(t, "success", finalStatus)
}

func TestAnomalyDetection(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Serve only 1 product — well below 20% of 100
	tinyFixture := `<!DOCTYPE html>
<html><body>
<ul class="products">
  <li class="product">
    <a href="/produto/only-one/" class="woocommerce-LoopProduct-link">
      <img class="wp-post-image" src="/img/x.jpg">
      <h2 class="woocommerce-loop-product__title">Only One Product</h2>
      <span class="woocommerce-Price-amount"><bdi>$ 500</bdi></span>
    </a>
  </li>
</ul>
</body></html>`

	srv := serveFixturePage(tinyFixture)
	defer srv.Close()

	storeID := insertTestStore(t, db, "Test Store Anomaly", srv.URL, srv.URL+"/?page={page}")
	defer cleanupStore(t, db, storeID)

	// Seed a past successful job with listings_found=100
	_, err := db.ExecContext(ctx, `
		INSERT INTO scrape_jobs (store_id, started_at, finished_at, status, listings_found)
		VALUES ($1, NOW()-INTERVAL '1 day', NOW()-INTERVAL '1 day' + INTERVAL '5 minutes', 'success', 100)
	`, storeID)
	require.NoError(t, err)

	alertCalled := false
	notifier := &captureNotifier{onAlert: func(storeName string, found, historical int) {
		alertCalled = true
		assert.Equal(t, 1, found)
		assert.Equal(t, 100, historical)
	}}

	worker := scraping.NewWorker(db, &scraping.DefaultParser{}, notifier)
	runner := scraping.NewRunner(db, worker)

	err = runner.RunStore(ctx, storeID)
	require.NoError(t, err)

	assert.True(t, alertCalled, "anomaly notifier should have been called")

	var status string
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT status FROM scrape_jobs WHERE store_id=$1 ORDER BY started_at DESC LIMIT 1`,
		storeID,
	).Scan(&status))
	assert.Equal(t, "partial", status)
}

// captureNotifier is a test notifier that calls an onAlert callback.
type captureNotifier struct {
	onAlert func(storeName string, found, historical int)
}

func (n *captureNotifier) SendScraperAlert(_ context.Context, storeName string, _ int64, found, historical int, _ error) error {
	if n.onAlert != nil {
		n.onAlert(storeName, found, historical)
	}
	return nil
}
