package scraping

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

// userAgents is a pool of real browser User-Agent strings for rotation.
var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64; rv:124.0) Gecko/20100101 Firefox/124.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:123.0) Gecko/20100101 Firefox/123.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3.1 Safari/605.1.15",
}

const maxPages = 50

// Worker executes a scrape job for a single store.
type Worker struct {
	db       *sql.DB
	parser   StoreParser
	notifier Notifier
	repo     *Repository
}

// NewWorker constructs a Worker with the provided dependencies.
func NewWorker(db *sql.DB, parser StoreParser, notifier Notifier) *Worker {
	return &Worker{
		db:       db,
		parser:   parser,
		notifier: notifier,
		repo:     &Repository{},
	}
}

// Run scrapes the given store, upserts all listings, and records a ScrapeJob.
// Returns an error only for fatal infrastructure failures; scraper-level errors
// (e.g. HTTP 500) are captured in the job record.
func (w *Worker) Run(ctx context.Context, sw StoreWithRule) error {
	jobID, err := w.repo.CreateJob(ctx, w.db, sw.Store.ID)
	if err != nil {
		return fmt.Errorf("creating job for store %q: %w", sw.Store.Name, err)
	}

	var (
		found   int
		updated int
		newCnt  int
		jobErr  *string
		status  = "success"
	)

	defer func() {
		// Ensure job is always finalised even on panic-recovery (not handled here)
		_ = w.repo.UpdateJob(ctx, w.db, jobID, status, found, updated, newCnt, jobErr)
	}()

	client := &http.Client{Timeout: 30 * time.Second}

	for page := 1; page <= maxPages; page++ {
		url := buildPageURL(sw.Rule.CatalogURLPattern, page)

		body, fetchErr := w.fetchPage(ctx, client, url, sw.Rule)
		if fetchErr != nil {
			msg := fetchErr.Error()
			jobErr = &msg
			status = "failed"
			_ = w.notifier.SendScraperAlert(ctx, sw.Store.Name, jobID, found, 0, fetchErr)
			return nil // job row already updated via defer
		}

		listings, parseErr := w.parser.Parse(strings.NewReader(body), sw.Rule)
		if parseErr != nil {
			msg := parseErr.Error()
			jobErr = &msg
			status = "failed"
			return nil
		}

		if len(listings) == 0 {
			// Empty page = end of pagination
			break
		}

		for _, raw := range listings {
			isNew, isUpd, upsertErr := UpsertListing(ctx, w.db, sw.Store.ID, raw, nil)
			if upsertErr != nil {
				// Log but continue — don't abort entire store scrape for one bad listing
				continue
			}
			found++
			if isNew {
				newCnt++
			} else if isUpd {
				updated++
			}
		}

		// Check if there is a next page
		if sw.Rule.PaginationSelector.Valid && sw.Rule.PaginationSelector.String != "" {
			if !pageHasNext(body, sw.Rule.PaginationSelector.String) {
				break
			}
		}

		// Respect per-store delay
		if sw.Rule.DelayMS > 0 {
			delay := time.Duration(sw.Rule.DelayMS) * time.Millisecond
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	// Anomaly check
	w.checkAnomaly(ctx, jobID, sw.Store.ID, sw.Store.Name, found, &status)

	return nil
}

// checkAnomaly sends an alert and updates the job status when the listing count
// is suspiciously low compared to the historical baseline.
func (w *Worker) checkAnomaly(ctx context.Context, jobID, storeID int64, storeName string, found int, status *string) {
	historical, err := w.repo.GetLastSuccessfulJobCount(ctx, w.db, storeID)
	if err != nil {
		// Non-fatal — anomaly check is best-effort
		return
	}

	if found == 0 {
		*status = "failed"
		_ = w.notifier.SendScraperAlert(ctx, storeName, jobID, found, historical, fmt.Errorf("zero listings returned"))
		return
	}

	if historical > 0 && found < historical/5 {
		*status = "partial"
		_ = w.notifier.SendScraperAlert(ctx, storeName, jobID, found, historical, nil)
	}
}

// fetchPage retrieves a single catalog page and returns the body as a string.
func (w *Worker) fetchPage(ctx context.Context, client *http.Client, url string, rule ScrapeRule) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("building request for %s: %w", url, err)
	}

	// Rotate User-Agent
	req.Header.Set("User-Agent", userAgents[rand.Intn(len(userAgents))]) //nolint:gosec
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "es-CO,es;q=0.9,en;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d fetching %s", resp.StatusCode, url)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading body from %s: %w", url, err)
	}

	return string(b), nil
}

// buildPageURL substitutes {page} placeholder in the catalog URL pattern.
func buildPageURL(pattern string, page int) string {
	if strings.Contains(pattern, "{page}") {
		return strings.ReplaceAll(pattern, "{page}", fmt.Sprintf("%d", page))
	}
	// If no placeholder, append page query param for page > 1
	if page == 1 {
		return pattern
	}
	sep := "?"
	if strings.Contains(pattern, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s%spage=%d", pattern, sep, page)
}

// pageHasNext checks if the HTML body contains the pagination selector.
func pageHasNext(body, selector string) bool {
	// Quick string scan: if the next-page link class/text doesn't appear, stop.
	// A full goquery parse is unnecessary here — we just need presence.
	doc, err := goQueryFromString(body)
	if err != nil {
		return false
	}
	return doc.Find(selector).Length() > 0
}
