// whatsapp_service.go — WhatsApp Business API integration.
//
// All order lifecycle notifications are sent through this service.
// If WHATSAPP_PHONE_ID is not set the service gracefully logs instead
// of sending, making local development easier.

package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/cjrdz/ishosBackEnd/internal/config"
)

type WhatsAppService struct {
	phoneID string
	token   string
	baseURL string
	enabled bool
	queue   chan whatsappJob
	closeMu sync.Mutex
	closed  bool
	wg      sync.WaitGroup
}

type whatsappJob struct {
	phone   string
	message string
}

func NewWhatsAppService(cfg *config.Config) *WhatsAppService {
	svc := &WhatsAppService{
		phoneID: cfg.WhatsAppPhoneID,
		token:   cfg.WhatsAppToken,
		baseURL: "https://graph.facebook.com/v18.0",
		enabled: cfg.WhatsAppPhoneID != "" && cfg.WhatsAppToken != "",
		queue:   make(chan whatsappJob, 256),
	}

	svc.wg.Add(1)
	go svc.startWorker()

	return svc
}

func (s *WhatsAppService) startWorker() {
	defer s.wg.Done()
	for job := range s.queue {
		s.sendWithRetry(job.phone, job.message)
	}
}

// whatsappPayload is the JSON body for the Meta API.
type whatsappPayload struct {
	MessagingProduct string `json:"messaging_product"`
	To               string `json:"to"`
	Type             string `json:"type"`
	Text             struct {
		Body string `json:"body"`
	} `json:"text"`
}

func (s *WhatsAppService) send(phone, message string) {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()
	if s.closed {
		return
	}

	select {
	case s.queue <- whatsappJob{phone: phone, message: message}:
	default:
		log.Warn().Str("to", maskPhone(phone)).Msg("WhatsApp queue full, dropping notification")
	}
}

// Close stops accepting new jobs, drains the queue, and waits for the worker
// to finish or for the context to expire. It is safe to call multiple times.
func (s *WhatsAppService) Close(ctx context.Context) error {
	if s == nil {
		return nil
	}

	s.closeMu.Lock()
	if s.closed {
		s.closeMu.Unlock()
		return nil
	}
	s.closed = true
	close(s.queue)
	s.closeMu.Unlock()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	if ctx == nil {
		<-done
		return nil
	}

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *WhatsAppService) sendWithRetry(phone, message string) {
	if !s.enabled {
		log.Info().
			Str("to", maskPhone(phone)).
			Int("message_len", len(message)).
			Msg("[WhatsApp DISABLED] Would send message")
		return
	}

	// Clean phone number
	to := strings.ReplaceAll(phone, "+", "")
	to = strings.ReplaceAll(to, " ", "")
	to = strings.ReplaceAll(to, "-", "")

	payload := whatsappPayload{
		MessagingProduct: "whatsapp",
		To:               to,
		Type:             "text",
	}
	payload.Text.Body = message

	body, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Msg("WhatsApp: failed to marshal payload")
		return
	}
	url := fmt.Sprintf("%s/%s/messages", s.baseURL, s.phoneID)

	const maxAttempts = 3
	backoff := time.Second
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Recreate the request each attempt — body reader cannot be replayed.
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			log.Error().Err(err).Msg("WhatsApp: failed to create request")
			return
		}
		req.Header.Set("Authorization", "Bearer "+s.token)
		req.Header.Set("Content-Type", "application/json")

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		resp, doErr := http.DefaultClient.Do(req.WithContext(ctx))
		cancel()

		if doErr != nil {
			log.Warn().Err(doErr).Int("attempt", attempt).Str("to", maskPhone(to)).Msg("WhatsApp: send failed")
		} else {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
				return // success
			}
			log.Warn().Int("status", resp.StatusCode).Int("attempt", attempt).Str("to", maskPhone(to)).Msg("WhatsApp: unexpected status")
		}

		if attempt < maxAttempts {
			time.Sleep(backoff)
			backoff *= 2 // exponential: 1s, 2s
		}
	}

	log.Error().Str("to", maskPhone(to)).Msg("WhatsApp: failed after all attempts")
}

func maskPhone(phone string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)

	if len(digits) <= 4 {
		return "****"
	}

	return strings.Repeat("*", len(digits)-4) + digits[len(digits)-4:]
}

// SendOrderCreated notifies customer their order was received.
func (s *WhatsAppService) SendOrderCreated(phone, orderNumber string, total float64, trackingURL string) {
	trackingLine := ""
	if trackingURL != "" {
		trackingLine = fmt.Sprintf("*Seguimiento seguro:* %s\n\n", trackingURL)
	}

	msg := fmt.Sprintf(
		"🧊 *Orden Recibida*\n\n"+
			"¡Gracias por tu pedido!\n\n"+
			"*Número de orden:* %s\n"+
			"*Total:* $%.2f\n\n"+
			"%s"+
			"Estamos revisando tu orden y te confirmaremos pronto. 🍦",
		orderNumber, total, trackingLine,
	)
	s.send(phone, msg)
}

// SendOrderApproved notifies customer their order was confirmed.
func (s *WhatsAppService) SendOrderApproved(phone, orderNumber, trackingURL string) {
	trackingLine := ""
	if trackingURL != "" {
		trackingLine = fmt.Sprintf("\n\n*Seguimiento seguro:* %s", trackingURL)
	}

	msg := fmt.Sprintf(
		"✅ *Orden Confirmada*\n\n"+
			"Tu orden *%s* ha sido confirmada y está en preparación.\n\n"+
			"Te avisaremos cuando esté lista. 🎉%s",
		orderNumber, trackingLine,
	)
	s.send(phone, msg)
}

// SendOrderRejected notifies customer their order was not accepted.
func (s *WhatsAppService) SendOrderRejected(phone, orderNumber, reason, trackingURL string) {
	trackingLine := ""
	if trackingURL != "" {
		trackingLine = fmt.Sprintf("\n\n*Seguimiento seguro:* %s", trackingURL)
	}

	msg := fmt.Sprintf(
		"❌ *Orden No Procesada*\n\n"+
			"Lamentamos informarte que tu orden *%s* no pudo ser procesada.\n\n"+
			"*Motivo:* %s\n\n"+
			"Por favor contáctanos para más información.%s",
		orderNumber, reason, trackingLine,
	)
	s.send(phone, msg)
}

// SendOrderReady notifies customer their order is ready for pickup.
func (s *WhatsAppService) SendOrderReady(phone, orderNumber, trackingURL string) {
	trackingLine := ""
	if trackingURL != "" {
		trackingLine = fmt.Sprintf("\n\n*Seguimiento seguro:* %s", trackingURL)
	}

	msg := fmt.Sprintf(
		"🎉 *¡Tu Orden Está Lista!*\n\n"+
			"Tu orden *%s* está lista para recoger.\n\n"+
			"¡Te esperamos! 🍦%s",
		orderNumber, trackingLine,
	)
	s.send(phone, msg)
}
