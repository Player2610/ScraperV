package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/protou/protou/internal/platform"
	"github.com/protou/protou/internal/scraping"
)

func main() {
	var (
		dryRun  = flag.Bool("dry-run", false, "Parse and print listings to stdout without writing to DB")
		storeID = flag.Int64("store", 0, "Run scraper for a single store ID (0 = all stores)")
	)
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := loadScraperConfig()

	if *dryRun {
		runDryMode(ctx, cfg, *storeID)
		return
	}

	db, err := platform.NewDB(cfg)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer db.Close()

	notifier := buildNotifier(cfg)
	worker := scraping.NewWorker(db, &scraping.DefaultParser{}, notifier)
	runner := scraping.NewRunner(db, worker)

	if *storeID != 0 {
		log.Printf("running scraper for store id=%d", *storeID)
		if runErr := runner.RunStore(ctx, *storeID); runErr != nil {
			log.Fatalf("scraper failed for store %d: %v", *storeID, runErr)
		}
	} else {
		log.Println("running scraper for all active stores")
		if runErr := runner.RunAll(ctx); runErr != nil {
			log.Fatalf("scraper run failed: %v", runErr)
		}
	}

	log.Println("scraper finished")
}

// loadScraperConfig loads configuration for the scraper.
// Unlike the API, the scraper only requires DATABASE_URL.
func loadScraperConfig() platform.Config {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}
	return platform.Config{
		DatabaseURL:  dbURL,
		ResendAPIKey: os.Getenv("RESEND_API_KEY"),
		// JWT and Maps keys are not needed by the scraper binary.
		JWTSecret:        os.Getenv("JWT_SECRET"),
		GoogleMapsAPIKey: os.Getenv("GOOGLE_MAPS_API_KEY"),
		Port:             "0",
	}
}

// buildNotifier returns an EmailNotifier when credentials are available,
// or a NoopNotifier for silent operation.
func buildNotifier(cfg platform.Config) scraping.Notifier {
	toEmail := os.Getenv("ALERT_EMAIL")
	if cfg.ResendAPIKey != "" && toEmail != "" {
		from := os.Getenv("NOTIFICATIONS_FROM")
		if from == "" {
			from = "protou alertas <alertas@protou.co>"
		}
		return &scraping.EmailNotifier{
			APIKey:  cfg.ResendAPIKey,
			ToEmail: toEmail,
			From:    from,
		}
	}
	log.Println("RESEND_API_KEY or ALERT_EMAIL not set — using no-op notifier")
	return &scraping.NoopNotifier{}
}

// runDryMode fetches and prints listings without any DB writes.
func runDryMode(ctx context.Context, cfg platform.Config, filterStoreID int64) {
	db, err := platform.NewDB(cfg)
	if err != nil {
		log.Fatalf("connecting to database: %v", err)
	}
	defer db.Close()

	repo := &scraping.Repository{}
	stores, err := repo.LoadActiveStoresWithRules(ctx, db)
	if err != nil {
		log.Fatalf("loading stores: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	parser := &scraping.DefaultParser{}

	for _, sw := range stores {
		if filterStoreID != 0 && sw.Store.ID != filterStoreID {
			continue
		}

		fmt.Printf("\n=== DRY RUN: %s ===\n", sw.Store.Name)

		pageURL := buildDryRunPageURL(sw.Rule.CatalogURLPattern)
		fmt.Printf("Fetching: %s\n", pageURL)

		body, fetchErr := dryFetchURL(ctx, client, pageURL)
		if fetchErr != nil {
			fmt.Printf("  ERROR: %v\n", fetchErr)
			continue
		}

		listings, parseErr := parser.Parse(strings.NewReader(body), sw.Rule)
		if parseErr != nil {
			fmt.Printf("  PARSE ERROR: %v\n", parseErr)
			continue
		}

		for i, l := range listings {
			price, _ := scraping.ParsePrice(l.PriceRaw)
			signal := scraping.ParseStockSignal(l.PriceRaw, l.StockRaw)
			fmt.Printf("  [%d] %-50s | price=%d | stock=%-16s | url=%s\n",
				i+1, l.Name, price, signal, l.ProductURL)
		}
		fmt.Printf("  --- %d listings found\n", len(listings))
	}
}

// buildDryRunPageURL returns the first-page URL from a pattern.
func buildDryRunPageURL(pattern string) string {
	return strings.ReplaceAll(pattern, "{page}", "1")
}

// dryFetchURL retrieves a URL and returns the body as a string.
func dryFetchURL(ctx context.Context, client *http.Client, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/123.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "es-CO,es;q=0.9,en;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading body: %w", err)
	}
	return string(b), nil
}
