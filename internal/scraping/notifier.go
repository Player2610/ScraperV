package scraping

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Notifier sends alert notifications for scraper events.
type Notifier interface {
	SendScraperAlert(ctx context.Context, storeName string, jobID int64, found, historical int, err error) error
}

// EmailNotifier sends scraper alerts via the Resend REST API.
type EmailNotifier struct {
	APIKey  string
	ToEmail string
	From    string // "Name <addr@domain>" — use dev/prod address accordingly
}

// resendRequest is the JSON body for the Resend emails.send endpoint.
type resendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	HTML    string   `json:"html"`
}

// SendScraperAlert sends an alert email when a scrape job is anomalous or failed.
func (n *EmailNotifier) SendScraperAlert(
	ctx context.Context,
	storeName string,
	jobID int64,
	found, historical int,
	scraperErr error,
) error {
	subject := fmt.Sprintf("[protou] Alerta scraper: %s", storeName)

	errText := "ninguno"
	if scraperErr != nil {
		errText = scraperErr.Error()
	}

	html := fmt.Sprintf(`
<h2>Alerta de scraper — %s</h2>
<table>
  <tr><td><b>Job ID</b></td><td>%d</td></tr>
  <tr><td><b>Tienda</b></td><td>%s</td></tr>
  <tr><td><b>Listings encontrados</b></td><td>%d</td></tr>
  <tr><td><b>Histórico (último éxito)</b></td><td>%d</td></tr>
  <tr><td><b>Error</b></td><td>%s</td></tr>
  <tr><td><b>Hora</b></td><td>%s</td></tr>
</table>
`,
		storeName,
		jobID, storeName,
		found, historical,
		errText,
		time.Now().Format(time.RFC1123),
	)

	from := n.From
	if from == "" {
		from = "protou alertas <alertas@protou.co>"
	}
	payload, err := json.Marshal(resendRequest{
		From:    from,
		To:      []string{n.ToEmail},
		Subject: subject,
		HTML:    html,
	})
	if err != nil {
		return fmt.Errorf("marshaling resend request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.resend.com/emails", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("creating resend HTTP request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+n.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending alert email via Resend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("resend returned HTTP %d for alert email", resp.StatusCode)
	}

	return nil
}

// NoopNotifier discards all alerts. Useful for dry-run and tests.
type NoopNotifier struct{}

// SendScraperAlert is a no-op implementation.
func (n *NoopNotifier) SendScraperAlert(_ context.Context, _ string, _ int64, _, _ int, _ error) error {
	return nil
}
