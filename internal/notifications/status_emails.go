package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// SendOrderConfirmed sends a "Tu pedido fue confirmado" email.
func (s *Service) SendOrderConfirmed(ctx context.Context, to, userName string, orderID int64) error {
	return s.sendSimpleEmail(ctx, to, userName, orderID,
		"order_confirmed",
		fmt.Sprintf("Tu pedido #%d fue confirmado — protou", orderID),
		fmt.Sprintf(`<html><body>
<h2>Hola, %s!</h2>
<p>Tu pedido <strong>#%d</strong> fue <strong>confirmado</strong>.</p>
<p>Estamos procesando tu compra. Te avisaremos cuando esté en camino.</p>
</body></html>`, userName, orderID),
	)
}

// SendOrderInDelivery sends a "Tu pedido está en camino" email.
func (s *Service) SendOrderInDelivery(ctx context.Context, to, userName string, orderID int64) error {
	return s.sendSimpleEmail(ctx, to, userName, orderID,
		"order_in_delivery",
		fmt.Sprintf("Tu pedido #%d está en camino — protou", orderID),
		fmt.Sprintf(`<html><body>
<h2>Hola, %s!</h2>
<p>Tu pedido <strong>#%d</strong> está <strong>en camino</strong>.</p>
<p>El mensajero se encuentra en ruta. Prepárate para recibirlo.</p>
</body></html>`, userName, orderID),
	)
}

// SendOrderDelivered sends a "Tu pedido fue entregado" email.
func (s *Service) SendOrderDelivered(ctx context.Context, to, userName string, orderID int64) error {
	return s.sendSimpleEmail(ctx, to, userName, orderID,
		"order_delivered",
		fmt.Sprintf("Tu pedido #%d fue entregado — protou", orderID),
		fmt.Sprintf(`<html><body>
<h2>Hola, %s!</h2>
<p>Tu pedido <strong>#%d</strong> fue <strong>entregado</strong>. ¡Disfrútalo!</p>
<p>Gracias por usar protou.</p>
</body></html>`, userName, orderID),
	)
}

// SendOrderCancelled sends a "Tu pedido fue cancelado" email.
func (s *Service) SendOrderCancelled(ctx context.Context, to, userName string, orderID int64) error {
	return s.sendSimpleEmail(ctx, to, userName, orderID,
		"order_cancelled",
		fmt.Sprintf("Tu pedido #%d fue cancelado — protou", orderID),
		fmt.Sprintf(`<html><body>
<h2>Hola, %s!</h2>
<p>Tu pedido <strong>#%d</strong> fue <strong>cancelado</strong>.</p>
<p>Si tienes alguna pregunta, contáctanos.</p>
</body></html>`, userName, orderID),
	)
}

// SendItemCancelled sends an item-cancelled notification email.
func (s *Service) SendItemCancelled(ctx context.Context, to, userName string, orderID int64, itemName string) error {
	return s.sendSimpleEmail(ctx, to, userName, orderID,
		"item_cancelled",
		fmt.Sprintf("Un ítem de tu pedido #%d fue cancelado — protou", orderID),
		fmt.Sprintf(`<html><body>
<h2>Hola, %s!</h2>
<p>El ítem <strong>%s</strong> de tu pedido <strong>#%d</strong> fue cancelado.</p>
<p>El total de tu pedido fue actualizado. Si tienes preguntas, contáctanos.</p>
</body></html>`, userName, itemName, orderID),
	)
}

// sendSimpleEmail sends a minimal HTML email via Resend and logs the result.
func (s *Service) sendSimpleEmail(ctx context.Context, to, userName string, orderID int64, event, subject, htmlBody string) error {
	if s.apiKey == "" {
		log.Printf("notifications: RESEND_API_KEY not set — skipping %s email", event)
		return nil
	}

	payload := map[string]interface{}{
		"from":    s.from,
		"to":      []string{to},
		"subject": subject,
		"html":    htmlBody,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("notifications: marshal %s payload: %v", event, err)
		s.logNotification(orderID, 0, event, "failed", err.Error())
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		log.Printf("notifications: create %s request: %v", event, err)
		s.logNotification(orderID, 0, event, "failed", err.Error())
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		log.Printf("notifications: %s request: %v", event, err)
		s.logNotification(orderID, 0, event, "failed", err.Error())
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		msg := fmt.Sprintf("resend returned %d", resp.StatusCode)
		log.Printf("notifications: %s: %s", event, msg)
		s.logNotification(orderID, 0, event, "failed", msg)
		return nil
	}

	s.logNotification(orderID, 0, event, "sent", "")
	return nil
}
