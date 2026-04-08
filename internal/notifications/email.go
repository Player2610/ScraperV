package notifications

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"runtime"
	"time"
)

// Service provides transactional email sending via Resend.
type Service struct {
	db         *sql.DB
	httpClient *http.Client
	tmpl       *template.Template
	apiKey     string // Resend API key — set once at construction
	from       string // "Name <addr@domain>" — differs between dev and prod
}

// NewService creates a new notifications Service.
// apiKey is the Resend API key; from is the From header value, e.g.
// "protou <pedidos@protou.co>" for prod or "protou DEV <pedidos+dev@protou.co>" for dev.
func NewService(db *sql.DB, apiKey, from string) *Service {
	tmpl := loadTemplate()
	return &Service{
		db:         db,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		tmpl:       tmpl,
		apiKey:     apiKey,
		from:       from,
	}
}

// Order and OrderItem are minimal types needed here to avoid import cycles.
// The full types live in internal/orders — we redeclare only what we need.
type Order struct {
	ID                      int64
	SubtotalCOP             int
	DeliveryFeeCOP          int
	TotalCOP                int
	PaymentMethod           string
	DeliveryAddressSnapshot AddressSnapshot
}

// AddressSnapshot mirrors orders.AddressSnapshot for email rendering.
type AddressSnapshot struct {
	FullAddress string
	Label       *string
	Reference   *string
}

// OrderItem is the minimal order item needed for the email.
type OrderItem struct {
	OrderID              int64
	ListingNameSnapshot  string
	ListingStoreSnapshot string
	PriceSnapshotCOP     int
	Quantity             int
}

// SendOrderCreated sends the order confirmation email via Resend.
// If sending fails, logs the error, records failure in notification_logs, and returns nil.
func (s *Service) SendOrderCreated(ctx context.Context, to, userName string, order Order, items []OrderItem) error {
	htmlBody, err := s.renderOrderCreated(userName, order, items)
	if err != nil {
		log.Printf("notifications: render template: %v", err)
		s.logNotification(order.ID, 0, "order_created", "failed", err.Error())
		return nil
	}

	if s.apiKey == "" {
		log.Printf("notifications: RESEND_API_KEY not set — skipping email")
		return nil
	}

	payload := map[string]interface{}{
		"from":    s.from,
		"to":      []string{to},
		"subject": fmt.Sprintf("Tu pedido #%d fue recibido — protou", order.ID),
		"html":    htmlBody,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("notifications: marshal resend payload: %v", err)
		s.logNotification(order.ID, 0, "order_created", "failed", err.Error())
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		log.Printf("notifications: create request: %v", err)
		s.logNotification(order.ID, 0, "order_created", "failed", err.Error())
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("notifications: resend request: %v", err)
		s.logNotification(order.ID, 0, "order_created", "failed", err.Error())
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		msg := fmt.Sprintf("resend returned %d", resp.StatusCode)
		log.Printf("notifications: %s", msg)
		s.logNotification(order.ID, 0, "order_created", "failed", msg)
		return nil
	}

	s.logNotification(order.ID, 0, "order_created", "sent", "")
	return nil
}

// logNotification inserts a notification_logs record.
// userID is 0 when not available (looked up from context elsewhere).
func (s *Service) logNotification(orderID int64, userID int64, event, status, errMsg string) {
	if s.db == nil {
		return
	}
	var errMsgVal interface{}
	if errMsg != "" {
		errMsgVal = errMsg
	}
	const q = `
		INSERT INTO notification_logs (order_id, user_id, channel, event, status, error_message)
		VALUES ($1, COALESCE(
			(SELECT user_id FROM orders WHERE id = $1),
			0
		), 'email', $2, $3, $4)
	`
	_, err := s.db.Exec(q, orderID, event, status, errMsgVal)
	if err != nil {
		log.Printf("notifications: log notification: %v", err)
	}
}

// renderOrderCreated renders the HTML email for order_created.
func (s *Service) renderOrderCreated(userName string, order Order, items []OrderItem) (string, error) {
	paymentLabels := map[string]string{
		"nequi":       "Nequi",
		"daviplata":   "Daviplata",
		"efectivo":    "Efectivo",
		"llaves_breve": "Llaves Breve",
	}
	label, ok := paymentLabels[order.PaymentMethod]
	if !ok {
		label = order.PaymentMethod
	}

	data := struct {
		UserName           string
		Order              Order
		Items              []OrderItem
		PaymentMethodLabel string
	}{
		UserName:           userName,
		Order:              order,
		Items:              items,
		PaymentMethodLabel: label,
	}

	var buf bytes.Buffer
	if err := s.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("notifications: execute template: %w", err)
	}
	return buf.String(), nil
}

func loadTemplate() *template.Template {
	funcMap := template.FuncMap{
		"FormatCOP": func(v int) string {
			// Format as thousands-separated integer, e.g. 12500 -> "12.500"
			s := fmt.Sprintf("%d", v)
			n := len(s)
			if n <= 3 {
				return s
			}
			var result []byte
			for i, c := range s {
				if i > 0 && (n-i)%3 == 0 {
					result = append(result, '.')
				}
				result = append(result, byte(c))
			}
			return string(result)
		},
	}

	// Try to load from filesystem relative to this file
	_, filename, _, _ := runtime.Caller(0)
	tmplPath := filepath.Join(filepath.Dir(filename), "templates", "order_created.html")

	tmpl, err := template.New("order_created.html").Funcs(funcMap).ParseFiles(tmplPath)
	if err != nil {
		// Fall back to embedded minimal template
		log.Printf("notifications: load template from %s: %v — using fallback", tmplPath, err)
		fallback := `<html><body>
<h2>Hola, {{.UserName}}! Tu pedido #{{.Order.ID}} fue recibido.</h2>
<p>Total: ${{FormatCOP .Order.TotalCOP}} COP</p>
<p>Pago: {{.PaymentMethodLabel}}</p>
</body></html>`
		tmpl = template.Must(template.New("order_created.html").Funcs(funcMap).Parse(fallback))
	}
	return tmpl
}
